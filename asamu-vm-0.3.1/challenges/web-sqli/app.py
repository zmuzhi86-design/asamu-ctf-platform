import html
import os
import sqlite3
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

FLAG = os.environ.get("ASAMU_FLAG") or os.environ.get("CM_FLAG")
if not FLAG:
    raise RuntimeError("ASAMU_FLAG is required")

database = sqlite3.connect(":memory:", check_same_thread=False)
database.executescript(
    """
CREATE TABLE books (title TEXT NOT NULL, author TEXT NOT NULL);
CREATE TABLE secrets (label TEXT NOT NULL, secret TEXT NOT NULL);
INSERT INTO books VALUES
  ('Web Security Basics', 'Alice'),
  ('SQL for Beginners', 'Bob'),
  ('Blue Team Handbook', 'Carol');
INSERT INTO secrets VALUES ('administrator-favorite', ?);
""".replace("?", "'" + FLAG.replace("'", "''") + "'")
)

PAGE = """<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>asamu 在线书店</title>
  <style>
    body{font-family:system-ui,sans-serif;background:#f3f8ff;color:#10233f;margin:0;padding:40px}
    .card{max-width:760px;margin:auto;background:#fff;border:3px solid #10233f;box-shadow:7px 7px #1677ff;padding:28px}
    input{box-sizing:border-box;width:72%;padding:12px;border:2px solid #1677ff}
    button{padding:12px 20px;background:#1677ff;color:#fff;border:2px solid #10233f;font-weight:800}
    li{padding:10px;border-bottom:1px solid #cfe2ff}.hint{background:#fff4b8;padding:10px}
  </style>
</head>
<body><main class="card">
  <h1>📚 asamu 在线书店</h1>
  <p class="hint">管理员最喜欢哪一本书？试着观察并改变检索语句。</p>
  <form><input name="q" value="__QUERY__" placeholder="输入书名"><button>搜索</button></form>
  <h2>检索结果</h2><ul>__RESULTS__</ul>
</main></body></html>"""


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        if parsed.path != "/":
            self.send_error(404)
            return

        query = parse_qs(parsed.query).get("q", [""])[0]
        try:
            sql = "SELECT title, author FROM books WHERE title LIKE '%" + query + "%'"
            results = database.execute(sql).fetchall()
        except sqlite3.Error as error:
            results = [("数据库提示", str(error))]

        items = "".join(
            f"<li><b>{html.escape(str(title))}</b> · {html.escape(str(author))}</li>"
            for title, author in results
        ) or "<li>没有找到结果</li>"
        body = PAGE.replace("__QUERY__", html.escape(query, quote=True)).replace("__RESULTS__", items).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        return


ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
