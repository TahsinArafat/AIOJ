"""
Custom Cloudflare Bypass Service with Login Support
Extends CloudflareBypassForScraping to maintain login sessions
"""
import json
import asyncio
from urllib.parse import urlparse, parse_qs
from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse, RedirectResponse
from DrissionPage import ChromiumPage, ChromiumOptions

app = FastAPI()

# Global browser session
browser_page = None
login_cookies = {}

def get_browser():
    global browser_page
    if browser_page is None:
        co = ChromiumOptions()
        co.headless()
        co.set_argument('--no-sandbox')
        co.set_argument('--disable-dev-shm-usage')
        co.set_argument('--disable-gpu')
        browser_page = ChromiumPage(co)
    return browser_page

@app.on_event("startup")
async def startup():
    get_browser()

@app.on_event("shutdown")
async def shutdown():
    global browser_page
    if browser_page:
        browser_page.quit()

@app.get("/cookies")
async def get_cookies(url: str):
    """Get cookies for a URL, including cf_clearance"""
    page = get_browser()
    try:
        page.get(url)
        await asyncio.sleep(2)  # Wait for Cloudflare challenge
        
        cookies = {}
        for cookie in page.cookies():
            cookies[cookie['name']] = cookie['value']
        
        return JSONResponse({
            "cookies": cookies,
            "user_agent": page.user_agent
        })
    except Exception as e:
        return JSONResponse({"error": str(e)}, status_code=500)

@app.post("/login")
async def login(request: Request):
    """Login to Codeforces and store session cookies"""
    global login_cookies
    
    body = await request.json()
    username = body.get("username", "")
    password = body.get("password", "")
    
    if not username or not password:
        return JSONResponse({"error": "username and password required"}, status_code=400)
    
    page = get_browser()
    try:
        # Navigate to login page
        page.get("https://codeforces.com/enter")
        await asyncio.sleep(3)  # Wait for Cloudflare challenge
        
        # Fill in login form
        handle_input = page.ele('#handleOrEmail')
        password_input = page.ele('#password')
        
        if not handle_input or not password_input:
            return JSONResponse({"error": "login form not found"}, status_code=500)
        
        handle_input.clear()
        handle_input.input(username)
        password_input.clear()
        password_input.input(password)
        
        # Check remember me
        remember = page.ele('#remember')
        if remember and not remember.prop('checked'):
            remember.click()
        
        # Submit form
        submit_btn = page.ele('css:input[type="submit"]')
        if submit_btn:
            submit_btn.click()
            await asyncio.sleep(3)  # Wait for login
        
        # Check if login succeeded
        current_url = page.url
        if '/enter' in current_url:
            # Still on login page - login failed
            error_msg = page.ele('.error')
            if error_msg:
                return JSONResponse({"error": f"login failed: {error_msg.text}"}, status_code=400)
            return JSONResponse({"error": "login failed"}, status_code=400)
        
        # Store cookies
        cookies = {}
        for cookie in page.cookies():
            cookies[cookie['name']] = cookie['value']
        
        login_cookies = cookies
        
        return JSONResponse({
            "status": "success",
            "cookies": cookies
        })
    except Exception as e:
        return JSONResponse({"error": str(e)}, status_code=500)

@app.get("/check-login")
async def check_login():
    """Check if currently logged in"""
    page = get_browser()
    try:
        page.get("https://codeforces.com/profile")
        await asyncio.sleep(2)
        
        # Check if redirected to login page
        if '/enter' in page.url:
            return JSONResponse({"logged_in": False})
        
        return JSONResponse({"logged_in": True})
    except Exception as e:
        return JSONResponse({"error": str(e)}, status_code=500)

@app.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
async def proxy(request: Request, path: str):
    """Proxy requests through the browser with Cloudflare bypass"""
    hostname = request.headers.get("x-hostname")
    if not hostname:
        return JSONResponse({"error": "x-hostname header required"}, status_code=400)
    
    page = get_browser()
    try:
        # Build target URL
        target_url = f"https://{hostname}/{path}"
        if request.url.query:
            target_url += f"?{request.url.query}"
        
        # Get request body
        body = None
        if request.method in ["POST", "PUT", "PATCH"]:
            body = await request.body()
        
        # Get headers
        headers = {}
        for key, value in request.headers.items():
            if key.lower() not in ['host', 'x-hostname', 'x-forwarded-for', 'x-real-ip']:
                headers[key] = value
        
        # Add login cookies if available
        if login_cookies:
            cookie_str = "; ".join(f"{k}={v}" for k, v in login_cookies.items())
            if 'cookie' in headers:
                headers['cookie'] += "; " + cookie_str
            else:
                headers['cookie'] = cookie_str
        
        # Make request through browser
        if request.method == "GET":
            page.get(target_url)
        elif request.method == "POST":
            # For POST requests, we need to use JavaScript to submit
            js_code = f"""
            fetch('{target_url}', {{
                method: 'POST',
                headers: {json.dumps(headers)},
                body: {json.dumps(body.decode() if body else '')}
            }}).then(r => r.text()).then(t => document.title = t);
            """
            page.run_js(js_code)
        
        await asyncio.sleep(2)
        
        # Get response
        content = page.html
        response_headers = {}
        
        return Response(
            content=content,
            headers=response_headers,
            media_type="text/html"
        )
    except Exception as e:
        return JSONResponse({"error": str(e)}, status_code=500)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
