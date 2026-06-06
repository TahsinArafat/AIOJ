import json, time, asyncio, os, sys, subprocess, tempfile
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import uvicorn

app = FastAPI()
SCREENSHOT_DIR = "/tmp/atcoder-screenshots"
PROFILE_DIR = "/tmp/atcoder-profile"
os.makedirs(SCREENSHOT_DIR, exist_ok=True)
os.makedirs(PROFILE_DIR, exist_ok=True)

BROWSER_SCRIPT = r'''
import json, time, os, sys
from cloakbrowser import launch_persistent_context

def ss(ctx, name):
    try:
        p = ctx.pages[0] if ctx.pages else None
        if p: p.screenshot(path=os.path.join(sys.argv[2], name))
    except: pass

def is_logged_in(page):
    try:
        url = page.url
        if "/home" in url:
            return True
        body = page.inner_text("body")
        return "Logout" in body or "ログアウト" in body or "Welcome" in body or "already signed in" in body
    except:
        return False

def fetch_latest_submission_id(page, contest_id):
    try:
        page.goto(f"https://atcoder.jp/contests/{contest_id}/submissions/me",
                   wait_until="load", timeout=30000)
        time.sleep(3)
        print(f"[info] submissions page url: {page.url}", flush=True)
        body = page.inner_text("body")[:300]
        print(f"[info] submissions page body: {body}", flush=True)
        all_links = page.query_selector_all('a[href]')
        print(f"[info] total links on page: {len(all_links)}", flush=True)
        for link in reversed(all_links):
            href = link.get_attribute("href") or ""
            if "/submissions/" in href:
                parts = href.rstrip("/").split("/")
                sub_id = parts[-1]
                if sub_id.isdigit():
                    print(f"[info] found submission ID: {sub_id}", flush=True)
                    return sub_id
                print(f"[info] non-numeric sub href: {href}", flush=True)
        print("[warn] no submission ID found in any link", flush=True)
    except Exception as e:
        print(f"[warn] fetch_latest_submission_id: {e}", flush=True)
    return ""

op           = sys.argv[1]
profile_dir  = sys.argv[2]
ss_dir       = sys.argv[3]
username     = sys.argv[4] if len(sys.argv) > 4 else ""
password     = sys.argv[5] if len(sys.argv) > 5 else ""

launch_kwargs = dict(
    headless=True,
    humanize=True,
    args=["--no-sandbox", "--disable-dev-shm-usage"],
)

ctx = launch_persistent_context(profile_dir, **launch_kwargs)

try:
    page = ctx.new_page() if not ctx.pages else ctx.pages[0]

    page.goto("https://atcoder.jp/login", wait_until="load", timeout=60000)

    print("[info] waiting for page + Turnstile to settle...", flush=True)
    time.sleep(10)

    logged_in = is_logged_in(page)
    print(f"[info] login_page logged_in={logged_in} url={page.url}", flush=True)

    if not logged_in:
        if not username or not password:
            print(json.dumps({"error": "not logged in and no credentials"}))
            sys.exit(0)

        ss(ctx, "01_login_page.png")

        found = False
        for attempt in range(20):
            if page.locator('input[name="username"]').count() > 0:
                found = True
                break
            time.sleep(2)
            print(f"[info] polling for login form, attempt {attempt+1}/20", flush=True)

        if not found:
            ss(ctx, "99_no_login_form.png")
            print(json.dumps({"error": "login form not found after 40s"}))
            sys.exit(0)

        print("[info] login form found, filling credentials", flush=True)
        page.locator('input[name="username"]').fill(username)
        page.locator('input[name="password"]').fill(password)
        time.sleep(1)
        ss(ctx, "02_before_login_submit.png")

        # Wait for Turnstile to be solved (token populated)
        print("[info] waiting for Turnstile token...", flush=True)
        turnstile_ok = False
        for attempt in range(30):
            try:
                token_el = page.query_selector('[name="cf-turnstile-response"]')
                if token_el:
                    val = token_el.get_attribute("value") or ""
                    if len(val) > 10:
                        print(f"[info] Turnstile token ready (len={len(val)})", flush=True)
                        turnstile_ok = True
                        break
                # Also check if there's an iframe from challenges.cloudflare.com
                frames = page.frames
                cf_frames = [f for f in frames if "challenges.cloudflare.com" in f.url]
                if cf_frames:
                    print(f"[info] Turnstile iframe found (count={len(cf_frames)}), waiting...", flush=True)
                else:
                    print(f"[info] attempt {attempt}: no Turnstile iframe, checking token...", flush=True)
            except Exception as e:
                print(f"[info] attempt {attempt}: {e}", flush=True)
            time.sleep(2)

        if not turnstile_ok:
            print("[warn] Turnstile token not ready, proceeding anyway...", flush=True)

        print("[info] clicking submit button", flush=True)
        submit_btn = page.locator('button[type="submit"], input[type="submit"]').first
        print(f"[info] submit button text: {submit_btn.inner_text() if submit_btn.count() > 0 else 'not found'}", flush=True)
        submit_btn.click()

        print("[info] waiting for Turnstile + login to complete...", flush=True)
        for i in range(30):
            time.sleep(3)
            current_url = page.url
            body_text = ""
            try:
                body_text = page.inner_text("body")[:200]
            except:
                pass
            print(f"[info] wait {i}: url={current_url} body_preview={body_text[:100]}", flush=True)
            if is_logged_in(page):
                break
            if "Invalid" in body_text or "Invalid username or password" in body_text:
                ss(ctx, "99_login_invalid.png")
                print(json.dumps({"error": "login failed - invalid credentials"}))
                sys.exit(0)

        ss(ctx, "03_after_login.png")

        if not is_logged_in(page):
            body_text = ""
            try:
                body_text = page.inner_text("body")[:500]
            except:
                pass
            ss(ctx, "99_login_failed.png")
            print(json.dumps({"error": f"login failed - check credentials. page: {body_text[:200]}"}))
            sys.exit(0)

        print("[info] login successful", flush=True)

    if op == "login":
        cookies = {}
        for c in ctx.cookies():
            if "atcoder" in c.get("domain", ""):
                cookies[c["name"]] = c["value"]
        print(json.dumps({"status": "ok", "message": "login successful", "cookies": cookies}))
        sys.exit(0)

    if op == "languages":
        # Navigate to any submit page to extract language options
        page.goto("https://atcoder.jp/contests/abc422/tasks/abc422_a", wait_until="load", timeout=60000)
        time.sleep(3)
        
        # Find and click a submit link or navigate directly to submit page
        submit_url = "https://atcoder.jp/contests/abc422/submit"
        page.goto(submit_url, wait_until="load", timeout=60000)
        time.sleep(3)
        
        if "/login" in page.url:
            print(json.dumps({"error": "redirected to login from submit page"}))
            sys.exit(0)
        
        # Wait for language select to appear
        found = False
        for attempt in range(20):
            if page.locator('select[name="data.LanguageId"]').count() > 0:
                found = True
                break
            time.sleep(2)
            print(f"[info] polling for language select, attempt {attempt+1}/20", flush=True)
        
        if not found:
            ss(ctx, "99_no_language_select.png")
            print(json.dumps({"error": "language select not found after 40s"}))
            sys.exit(0)
        
        # Extract all language options
        select_el = page.locator('select[name="data.LanguageId"]')
        options = select_el.evaluate("""el => {
            const result = [];
            for (const opt of el.options) {
                if (opt.value) result.push({value: opt.value, text: opt.textContent.trim()});
            }
            return result;
        }""")
        print(f"[info] detected {len(options)} languages", flush=True)
        print(json.dumps({"status": "ok", "languages": options}))
        sys.exit(0)

    if op == "submit":
        contest_id = sys.argv[6] if len(sys.argv) > 6 else ""
        problem_id = sys.argv[7] if len(sys.argv) > 7 else ""
        source_file = sys.argv[8] if len(sys.argv) > 8 else ""
        lang_id = sys.argv[9] if len(sys.argv) > 9 else "5001"

        source_code = ""
        if source_file and os.path.exists(source_file):
            with open(source_file, "r") as f:
                source_code = f.read()

        submit_url = f"https://atcoder.jp/contests/{contest_id}/submit"
        page.goto(submit_url, wait_until="load", timeout=60000)
        time.sleep(3)
        ss(ctx, "04_submit_page.png")

        if "/login" in page.url:
            print(json.dumps({"error": "redirected to login from submit page"}))
            sys.exit(0)

        found = False
        for attempt in range(20):
            if page.locator('select[name="data.LanguageId"]').count() > 0:
                found = True
                break
            time.sleep(2)
            print(f"[info] polling for submit form, attempt {attempt+1}/20", flush=True)

        if not found:
            ss(ctx, "99_no_submit_form.png")
            print(json.dumps({"error": "submit form not found after 40s"}))
            sys.exit(0)

        print("[info] submit form found, filling", flush=True)

        form_info = page.evaluate("""() => {
            const info = {selects: [], textareas: [], editors: []};
            document.querySelectorAll('select').forEach(s => {
                const opts = [];
                for (let i = 0; i < Math.min(s.options.length, 5); i++) {
                    opts.push({value: s.options[i].value, text: s.options[i].textContent.trim()});
                }
                info.selects.push({name: s.name, id: s.id, optCount: s.options.length, sample: opts});
            });
            document.querySelectorAll('textarea').forEach(t => {
                const parent = t.parentElement;
                info.textareas.push({
                    name: t.name, id: t.id,
                    parentClass: parent ? parent.className : '',
                    parentTag: parent ? parent.tagName : '',
                    cm: !!t.closest('.CodeMirror'),
                    ace: !!t.closest('.ace_editor'),
                    monaco: !!t.closest('.monaco-editor'),
                    visible: t.offsetParent !== null,
                });
            });
            if (window.CodeMirror) info.editors.push('CodeMirror-global');
            if (window.ace) info.editors.push('ace-global');
            if (window.monaco) info.editors.push('monaco-global');
            const cmInstances = document.querySelectorAll('.CodeMirror');
            if (cmInstances.length) info.editors.push('CodeMirror-instances:' + cmInstances.length);
            const aceInstances = document.querySelectorAll('.ace_editor');
            if (aceInstances.length) info.editors.push('ace-instances:' + aceInstances.length);
            return info;
        }""")
        print(f"[info] form_info: selects={len(form_info['selects'])}, textareas={len(form_info['textareas'])}, editors={form_info['editors']}", flush=True)

        task_selects = [s for s in form_info["selects"] if s["name"] and ("task" in s["name"].lower() or "problem" in s["name"].lower() or "prob" in s["name"].lower())]
        if not task_selects:
            task_selects = [s for s in form_info["selects"] if s["optCount"] < 30 and s["name"] != "data.LanguageId"]
        if task_selects:
            task_select = page.locator(f'select[name="{task_selects[0]["name"]}"]')
            for opt in page.evaluate(f"""() => {{
                const s = document.querySelector('select[name="{task_selects[0]["name"]}"]');
                return Array.from(s.options).map(o => ({{value: o.value, text: o.textContent.trim()}}));
            }}"""):
                if problem_id.lower() in opt["value"].lower() or problem_id.lower() in opt["text"].lower():
                    task_select.select_option(value=opt["value"])
                    print(f"[info] selected task: {opt['text']}", flush=True)
                    break
            else:
                print(f"[warn] problem_id '{problem_id}' not found in task dropdown", flush=True)
        else:
            print(f"[info] no task dropdown found, contest URL may pre-select it", flush=True)
        time.sleep(0.5)

        select_el = page.locator('select[name="data.LanguageId"]')
        options = select_el.evaluate("""el => {
            const result = [];
            for (const opt of el.options) {
                if (opt.value) result.push({value: opt.value, text: opt.textContent.trim()});
            }
            return result;
        }""")
        print(f"[info] available languages: {len(options)}", flush=True)

        OLD_ID_TO_NAME = {
            "5001": ["Go", "go"], "5002": ["Java", "java"], "5003": ["C ", "C\n"],
            "5013": ["JavaScript", "Node.js", "js"], "5014": ["Rust", "rust"],
            "5028": ["Python", "python"], "5004": ["C#", "csharp"],
        }

        matched = False
        for opt in options:
            if opt["value"] == lang_id:
                select_el.select_option(value=lang_id)
                matched = True
                print(f"[info] selected exact: {opt['text']}", flush=True)
                break

        if not matched:
            name_hints = OLD_ID_TO_NAME.get(lang_id, [])
            for opt in options:
                for hint in name_hints:
                    if hint.lower() in opt["text"].lower():
                        select_el.select_option(value=opt["value"])
                        matched = True
                        print(f"[info] selected by name: {opt['text']}", flush=True)
                        break
                if matched:
                    break

        if not matched and options:
            select_el.select_option(value=options[0]["value"])
            print(f"[info] fallback to first lang: {options[0]['text']}", flush=True)
        time.sleep(1)

        source_filled = page.evaluate("""(code) => {
            const ta = document.querySelector('textarea[name="sourceCode"]');
            if (!ta) return 'no textarea';

            if (window.CodeMirror) {
                const cmEl = document.querySelector('.CodeMirror');
                if (cmEl && cmEl.CodeMirror) { cmEl.CodeMirror.setValue(code); return 'codemirror-global'; }
            }
            let el = ta;
            while (el) {
                if (el.CodeMirror) { el.CodeMirror.setValue(code); return 'codemirror-walk'; }
                if (el.classList && el.classList.contains('CodeMirror')) { el.CodeMirror.setValue(code); return 'codemirror-class'; }
                el = el.parentElement;
            }
            if (window.ace) {
                const aceEl = document.querySelector('.ace_editor');
                if (aceEl) {
                    const editor = window.ace.edit(aceEl);
                    editor.setValue(code, -1);
                    // Sync Ace editor content to hidden textarea so form POST includes it
                    ta.value = code;
                    ta.dispatchEvent(new Event('input', {bubbles: true}));
                    return 'ace';
                }
            }
            if (window.monaco) {
                const models = window.monaco.editor.getModels();
                if (models.length) { models[0].setValue(code); return 'monaco'; }
            }
            ta.value = code;
            ta.dispatchEvent(new Event('input', {bubbles: true}));
            ta.dispatchEvent(new Event('change', {bubbles: true}));
            return 'textarea-raw';
        }""", source_code)
        print(f"[info] source filled via: {source_filled}", flush=True)
        time.sleep(1)
        ss(ctx, "05_before_submit.png")
        page.locator('button[type="submit"], input[type="submit"]').first.click()
        
        try:
            page.wait_for_load_state("load", timeout=15000)
        except:
            pass
        time.sleep(3)
        ss(ctx, "06_after_submit.png")

        if "/login" in page.url:
            print(json.dumps({"error": "redirected to login after submit"}))
            sys.exit(0)

        after_url = page.url
        print(f"[info] after submit url: {after_url}", flush=True)
        if "/submissions/" in after_url and after_url.rstrip("/").split("/")[-1].isdigit():
            sub_id = after_url.rstrip("/").split("/")[-1]
            print(json.dumps({"status": "ok", "submission_id": sub_id}))
            sys.exit(0)

        page.goto(f"https://atcoder.jp/contests/{contest_id}/submissions/me",
                   wait_until="load", timeout=30000)
        time.sleep(5)
        page.wait_for_load_state("networkidle", timeout=15000)
        time.sleep(2)
        print(f"[info] submissions page url: {page.url}", flush=True)

        sub_id = page.evaluate("""() => {
            const rows = document.querySelectorAll('table tbody tr');
            for (const row of rows) {
                const link = row.querySelector('td:nth-child(6) a, td a[href*="/submissions/"]');
                if (link) {
                    const m = link.href.match(/\\/submissions\\/(\\d+)/);
                    if (m) return m[1];
                }
            }
            const links = document.querySelectorAll('a[href*="/submissions/"]');
            for (const link of links) {
                const m = link.href.match(/\\/submissions\\/(\\d+)/);
                if (m && m[1].match(/^\\d{7,}$/)) return m[1];
            }
            return null;
        }""")
        if sub_id:
            print(json.dumps({"status": "ok", "submission_id": sub_id}))
            sys.exit(0)

        all_links = page.query_selector_all('a[href]')
        print(f"[info] total links: {len(all_links)}", flush=True)
        for link in all_links:
            href = link.get_attribute("href") or ""
            if "/submissions/" in href and href.rstrip("/").split("/")[-1].isdigit():
                sub_id = href.rstrip("/").split("/")[-1]
                print(json.dumps({"status": "ok", "submission_id": sub_id}))
                sys.exit(0)
        print(json.dumps({"status": "ok", "message": "submitted but no submission ID found"}))
        sys.exit(0)

finally:
    ctx.close()
'''


def _run_browser(op, *args):
    with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as f:
        f.write(BROWSER_SCRIPT)
        sp = f.name
    cmd = [sys.executable, sp, op, PROFILE_DIR, SCREENSHOT_DIR] + list(args)
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
    os.unlink(sp)
    print(f"BROWSER stdout:\n{r.stdout}\nBROWSER stderr:\n{r.stderr}", flush=True)
    if r.returncode != 0:
        return {"error": f"crash: {r.stderr[-500:]}", "stdout": r.stdout[-1000:]}
    try:
        last = r.stdout.strip().split("\n")[-1]
        return json.loads(last)
    except:
        return {"error": "parse error", "stdout": r.stdout[-2000:]}


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/languages")
async def languages():
    loop = asyncio.get_event_loop()
    r = await loop.run_in_executor(None, _run_browser, "languages")
    if "error" in r:
        return JSONResponse(r, status_code=400)
    return r


@app.post("/login")
async def login(request: Request):
    body = await request.json()
    u = body.get("username", "")
    p = body.get("password", "")
    if not u or not p:
        return JSONResponse({"error": "username and password required"}, status_code=400)
    loop = asyncio.get_event_loop()
    r = await loop.run_in_executor(None, _run_browser, "login", u, p)
    if "error" in r:
        return JSONResponse(r, status_code=400)
    return r


@app.post("/submit")
async def submit(request: Request):
    body = await request.json()
    contest_id = body.get("contest_id", "")
    problem_id = body.get("problem_id", "")
    source_code = body.get("source_code", "")
    lang_id = str(body.get("lang_id", "5001"))
    username = body.get("username", "")
    password = body.get("password", "")
    if not contest_id or not problem_id or not source_code:
        return JSONResponse({"error": "contest_id, problem_id, and source_code required"}, status_code=400)
    if not username or not password:
        return JSONResponse({"error": "username and password required"}, status_code=400)

    with tempfile.NamedTemporaryFile(mode="w", suffix=".go", delete=False) as f:
        f.write(source_code)
        src = f.name

    loop = asyncio.get_event_loop()
    r = await loop.run_in_executor(None, _run_browser, "submit", username, password,
                                     contest_id, problem_id, src, lang_id)
    os.unlink(src)
    if "error" in r:
        return JSONResponse(r, status_code=400)
    return r


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=int(os.environ.get("PORT", 8004)))
