import json, time, asyncio, os, sys, subprocess, tempfile, requests
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import uvicorn

app = FastAPI()
SCREENSHOT_DIR = "/tmp/cf-screenshots"
PROFILE_DIR = "/tmp/cb-profile"
os.makedirs(SCREENSHOT_DIR, exist_ok=True)
os.makedirs(PROFILE_DIR, exist_ok=True)

PROXY = os.environ.get("CF_PROXY", "")

BROWSER_SCRIPT = r'''
import json, time, os, sys, requests
from cloakbrowser import launch_persistent_context

def ss(ctx, name):
    try:
        p = ctx.pages[0] if ctx.pages else None
        if p: p.screenshot(path=os.path.join(sys.argv[2], name))
    except: pass

def is_logged_in(page):
    try: return "Logout" in page.inner_text("body")
    except: return False

def fetch_sid(handle):
    for _ in range(3):
        try:
            r = requests.get(
                f"https://codeforces.com/api/user.status?handle={handle}&count=20",
                headers={"User-Agent": "Mozilla/5.0"}, timeout=10,
            )
            if r.status_code == 200:
                d = r.json()
                if d.get("status") == "OK" and d.get("result"):
                    return str(d["result"][0].get("id", ""))
        except: pass
        time.sleep(3)
    return ""

op           = sys.argv[1]
profile_dir  = sys.argv[2]
ss_dir       = sys.argv[3]
proxy        = sys.argv[4]
username     = sys.argv[5] if len(sys.argv) > 5 else ""
password     = sys.argv[6] if len(sys.argv) > 6 else ""

if op == "submit":
    source_path  = sys.argv[7]  if len(sys.argv) > 7  else ""
    problem_code = sys.argv[8]  if len(sys.argv) > 8  else ""
    lang_id      = sys.argv[9]  if len(sys.argv) > 9  else "54"
    handle       = sys.argv[10] if len(sys.argv) > 10 else username

launch_kwargs = dict(
    headless=False,
    humanize=True,
    args=["--no-sandbox", "--disable-dev-shm-usage"],
)
if proxy:
    launch_kwargs["proxy"] = proxy
    launch_kwargs["geoip"] = True

ctx = launch_persistent_context(profile_dir, **launch_kwargs)

try:
    page = ctx.new_page() if not ctx.pages else ctx.pages[0]

    page.goto("https://codeforces.com/", wait_until="domcontentloaded")
    time.sleep(4)
    ss(ctx, "00_home.png")
    logged_in = is_logged_in(page)
    print(f"[info] home logged_in={logged_in}", flush=True)

    if not logged_in:
        if not username or not password:
            print(json.dumps({"error": "not logged in and no credentials"}))
            sys.exit(0)

        page.goto("https://codeforces.com/enter", wait_until="domcontentloaded")
        time.sleep(4)
        ss(ctx, "01_enter.png")

        try:
            page.wait_for_selector("#handleOrEmail", state="visible", timeout=60000)
        except:
            body = page.inner_text("body")
            ss(ctx, "99_no_form.png")
            print(json.dumps({"error": f"login form not found. CF blocked: {'Performing security' in body}"}))
            sys.exit(0)

        page.locator("#handleOrEmail").fill(username)
        page.locator("#password").fill(password)
        try:
            rm = page.locator("#remember")
            if rm.is_visible(timeout=1000) and not rm.is_checked():
                rm.click()
        except: pass
        page.locator('input[type="submit"]').click()
        time.sleep(8)
        ss(ctx, "02_after_login.png")

        if not is_logged_in(page):
            ss(ctx, "99_login_failed.png")
            print(json.dumps({"error": "login failed"}))
            sys.exit(0)

        print("[info] login successful", flush=True)

    if op == "login":
        print(json.dumps({"status": "ok", "message": "login successful"}))
        sys.exit(0)

    if op == "submit":
        page.goto("https://codeforces.com/problemset/submit", wait_until="domcontentloaded")
        time.sleep(4)
        ss(ctx, "03_submit.png")

        if "enter" in page.url.lower():
            print(json.dumps({"error": "redirected to login"}))
            sys.exit(0)

        try:
            page.wait_for_selector('input[name="submittedProblemCode"]', state="visible", timeout=15000)
        except:
            ss(ctx, "99_no_submit_form.png")
            print(json.dumps({"error": "submit form not found"}))
            sys.exit(0)

        page.fill('input[name="submittedProblemCode"]', problem_code)
        time.sleep(0.5)
        page.select_option('select[name="programTypeId"]', lang_id)
        time.sleep(0.5)
        page.set_input_files('input[name="sourceFile"]', source_path)
        time.sleep(1)
        ss(ctx, "04_before_submit.png")
        page.click('input[type="submit"][value="Submit"]')
        time.sleep(10)
        ss(ctx, "05_after_submit.png")

        for _ in range(10):
            sid = fetch_sid(handle)
            if sid:
                print(json.dumps({"status": "ok", "submission_id": sid}))
                sys.exit(0)
            time.sleep(5)

        ss(ctx, "99_no_sid.png")
        print(json.dumps({"status": "ok", "message": "submitted but no submission ID"}))
        sys.exit(0)

finally:
    ctx.close()
'''


def _run_browser(op, proxy, *args):
	with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as f:
		f.write(BROWSER_SCRIPT)
		sp = f.name
	cmd = [sys.executable, sp, op, PROFILE_DIR, SCREENSHOT_DIR, proxy] + list(args)
	r = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
	os.unlink(sp)
	if r.returncode != 0:
		return {"error": f"crash: {r.stderr[-500:]}"}
	try:
		last = r.stdout.strip().split("\n")[-1]
		return json.loads(last)
	except:
		return {"error": "parse error", "stdout": r.stdout[-2000:]}


@app.get("/health")
async def health():
    return {"status": "ok", "proxy": bool(PROXY)}


@app.post("/login")
async def login(request: Request):
	body = await request.json()
	u = body.get("username", "")
	p = body.get("password", "")
	proxy = body.get("proxy")
	if proxy is None:
		proxy = PROXY
	if not u or not p:
		return JSONResponse({"error": "username and password required"}, status_code=400)
	loop = asyncio.get_event_loop()
	r = await loop.run_in_executor(None, _run_browser, "login", proxy, u, p)
	if "error" in r:
		return JSONResponse(r, status_code=400)
	return r


@app.post("/submit")
async def submit(request: Request):
	body = await request.json()
	pc = body.get("problem_code", "")
	sc = body.get("source_code", "")
	li = str(body.get("lang_id", "70"))
	h  = body.get("handle", "")
	u  = body.get("username", "")
	p  = body.get("password", "")
	proxy = body.get("proxy")
	if proxy is None:
		proxy = PROXY
	if not pc or not sc:
		print("400: pc or sc missing", body, flush=True)
		return JSONResponse({"error": "problem_code and source_code required"}, status_code=400)
	if not u or not p:
		print("400: u or p missing", body, flush=True)
		return JSONResponse({"error": "username and password required"}, status_code=400)
	with tempfile.NamedTemporaryFile(mode="w", suffix=".cpp", delete=False) as f:
		f.write(sc)
		src = f.name
	loop = asyncio.get_event_loop()
	r = await loop.run_in_executor(None, _run_browser, "submit", proxy, u, p, src, pc, li, h or u)
	os.unlink(src)
	if "error" in r:
		return JSONResponse(r, status_code=400)
	return r


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=int(os.environ.get("PORT", 8002)))
