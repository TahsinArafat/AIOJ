import json, time, asyncio, os, sys, subprocess, tempfile, requests
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import uvicorn

app = FastAPI()
SCREENSHOT_DIR = "/tmp/cf-screenshots"
os.makedirs(SCREENSHOT_DIR, exist_ok=True)

BROWSER_SCRIPT = r'''
import json, time, os, sys, requests
from cloakbrowser import launch

def ss(p, name):
    try: p.screenshot(path=os.path.join(sys.argv[2], name))
    except: pass

def is_logged_in(p):
    try: return "Logout" in p.inner_text('body')
    except: return False

def fetch_sid(handle):
    for _ in range(3):
        try:
            r = requests.get(f"https://codeforces.com/api/user.status?handle={handle}&count=20", headers={"User-Agent":"Mozilla/5.0"}, timeout=10)
            if r.status_code == 200:
                d = r.json()
                if d.get("status") == "OK" and d.get("result"):
                    return str(d["result"][0].get("id",""))
        except: pass
        time.sleep(3)
    return ""

op = sys.argv[1]
username = ""
password = ""
if op == "login":
    username = sys.argv[3]
    password = sys.argv[4]
elif op == "submit":
    source_path = sys.argv[3]
    problem_code = sys.argv[4]
    lang_id = sys.argv[5]
    handle = sys.argv[6]
    username = sys.argv[7] if len(sys.argv) > 7 else ""
    password = sys.argv[8] if len(sys.argv) > 8 else ""

browser = launch(headless=False, humanize=True)
ctx = browser.new_context()
page = ctx.new_page()

try:
    page.goto("https://codeforces.com/enter", wait_until="domcontentloaded")
    time.sleep(3)
    ss(page, "01_enter_page.png")

    if "enter" not in page.url.lower():
        page.goto("https://codeforces.com/", wait_until="domcontentloaded")
        time.sleep(2)
        if not is_logged_in(page):
            page.goto("https://codeforces.com/enter", wait_until="domcontentloaded")
            time.sleep(3)

    if "enter" in page.url.lower():
        for attempt in range(12):
            time.sleep(5)
            ss(page, f"02_login_attempt_{attempt}.png")
            try:
                hi = page.locator('#handleOrEmail')
                if hi.is_visible(timeout=2000):
                    hi.fill(username)
                    page.locator('#password').fill(password)
                    try:
                        rm = page.locator('#remember')
                        if rm.is_visible(timeout=1000) and not rm.is_checked():
                            rm.click()
                    except: pass
                    page.locator('input[type="submit"]').click()
                    time.sleep(8)
                    ss(page, "03_after_login_click.png")
                    if is_logged_in(page):
                        break
                    time.sleep(3)
            except:
                pass
        else:
            ss(page, "99_login_failed.png")
            print(json.dumps({"error":"login failed after 12 attempts"}))
            sys.exit(0)

    page.goto("https://codeforces.com/", wait_until="domcontentloaded")
    time.sleep(2)
    if not is_logged_in(page):
        ss(page, "99_not_logged_in.png")
        print(json.dumps({"error":"not logged in after login attempt"}))
        sys.exit(0)

    if op == "login":
        print(json.dumps({"status":"ok","message":"login successful"}))
        sys.exit(0)

    if op == "submit":
        page.goto("https://codeforces.com/problemset/submit", wait_until="domcontentloaded")
        time.sleep(5)
        ss(page, "04_submit_page.png")

        for submit_attempt in range(6):
            if "enter" in page.url.lower():
                print(json.dumps({"error":"not logged in - redirected to enter"}))
                sys.exit(0)
            try:
                page.wait_for_selector('input[name="submittedProblemCode"]', state="visible", timeout=3000)
                break
            except:
                time.sleep(3)
                ss(page, f"04b_submit_retry_{submit_attempt}.png")
        else:
            ss(page, "99_submit_form_not_found.png")
            print(json.dumps({"error":"submit form not found after 6 attempts"}))
            sys.exit(0)

        page.fill('input[name="submittedProblemCode"]', problem_code)
        time.sleep(0.5)
        page.select_option('select[name="programTypeId"]', lang_id)
        time.sleep(0.5)
        page.set_input_files('input[name="sourceFile"]', source_path)
        time.sleep(1)
        ss(page, "05_before_submit_click.png")
        page.click('input[type="submit"][value="Submit"]')
        time.sleep(10)
        ss(page, "06_after_submit.png")

        for retry in range(10):
            sid = fetch_sid(handle)
            if sid:
                print(json.dumps({"status":"ok","submission_id":sid}))
                sys.exit(0)
            time.sleep(5)

        ss(page, "99_submission_id_not_found.png")
        print(json.dumps({"status":"ok","message":"submitted but no submission ID"}))
        sys.exit(0)

finally:
    browser.close()
'''

def _run_browser(op, *args):
    with tempfile.NamedTemporaryFile(mode='w', suffix='.py', delete=False) as f:
        f.write(BROWSER_SCRIPT)
        sp = f.name
    cmd = [sys.executable, sp, op, SCREENSHOT_DIR] + list(args)
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=240)
    os.unlink(sp)
    if r.returncode != 0:
        return {"error": f"crash: {r.stderr[-500:]}"}
    try:
        return json.loads(r.stdout.strip().split('\n')[-1])
    except:
        return {"error":"parse error", "stdout": r.stdout[-2000:]}

@app.get("/health")
async def health():
    return {"status":"ok"}

@app.post("/login")
async def login(request: Request):
    body = await request.json()
    u = body.get("username",""); p = body.get("password","")
    if not u or not p:
        return JSONResponse({"error":"username and password required"}, status_code=400)
    loop = asyncio.get_event_loop()
    r = await loop.run_in_executor(None, _run_browser, "login", u, p)
    if "error" in r: return JSONResponse(r, status_code=400)
    return r

@app.post("/submit")
async def submit(request: Request):
    body = await request.json()
    pc = body.get("problem_code",""); sc = body.get("source_code","")
    li = body.get("lang_id","70"); h = body.get("handle","")
    u = body.get("username",""); p = body.get("password","")
    if not pc or not sc or not u or not p:
        return JSONResponse({"error":"problem_code, source_code, username, password required"}, status_code=400)
    with tempfile.NamedTemporaryFile(mode='w', suffix='.cpp', delete=False) as f:
        f.write(sc); sp = f.name
    loop = asyncio.get_event_loop()
    r = await loop.run_in_executor(None, _run_browser, "submit", sp, pc, str(li), h, u, p)
    os.unlink(sp)
    if "error" in r: return JSONResponse(r, status_code=400)
    return r

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8002)
