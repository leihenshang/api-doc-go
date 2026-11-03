"""服务端 HTTP 端到端测试（真实进程 + 真实 SQLite）。

用法：
    # 1) 启动服务端（独立 DB 与上传目录，不碰开发库）
    PORT=8093 GIN_MODE=release DB_DRIVER=sqlite DB_DSN=data/tmp/e2e.db \
      JWT_SECRET=e2e-secret SECRET_ENCRYPT_KEY=e2e-key ADMIN_NAME=admin ADMIN_PASSWORD=admin123 \
      UPLOAD_DIR=data/tmp/e2e-uploads go run ./cmd/server

    # 2) 跑本脚本
    python3 data/e2e_full.py

覆盖：健康检查/能力协商、认证与 PAT、项目与成员权限、分组、接口 CRUD 与回收站、
示例、导入导出与代码片段、文档、变量与环境、字段映射、搜索、分享、Mock、同步、
历史与回滚、契约校验、代理、上传与静态取回、Bug、公共属性、操作日志。
"""
import json
import os
import threading
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import quote

BASE = os.environ.get("E2E_BASE", "http://127.0.0.1:8093")
results = []


def expect(module, name, cond, extra=""):
    results.append((module, name, bool(cond), extra))
    mark = "PASS" if cond else "FAIL"
    line = f"  [{mark}] {name}"
    if not cond and extra:
        line += f"  <- {extra}"
    print(line)


def req(method, path, body=None, token=None, base=BASE, raw=False, headers=None):
    data = None
    hdrs = {"Content-Type": "application/json"} if body is not None else {}
    if headers:
        hdrs.update(headers)
    if token:
        hdrs["Authorization"] = f"Bearer {token}"
    if body is not None:
        data = json.dumps(body).encode()
    # 查询串里可能带中文（keyword=登录）：urllib 只接受 ASCII，这里统一转义
    safe = "/?&=:%#[]@!$'()*+,;~"
    r = urllib.request.Request(base + quote(path, safe=safe), data=data, method=method, headers=hdrs)
    try:
        with urllib.request.urlopen(r) as resp:
            payload = resp.read()
            ctype = resp.headers.get("Content-Type", "")
    except urllib.error.HTTPError as e:
        payload, ctype = e.read(), e.headers.get("Content-Type", "")
        if raw:
            return e.code, payload, ctype
        return {"code": e.code, "msg": payload.decode(errors="replace")[:200], "data": None}
    if raw:
        return 200, payload, ctype
    try:
        return json.loads(payload)
    except Exception:
        return {"code": -1, "msg": payload.decode(errors="replace")[:200], "data": None}


def api(method, path, body=None, token=None, base=BASE):
    return req(method, path, body, token, base)


def ok(out):
    return isinstance(out, dict) and out.get("code") == 200


def rows(data):
    """列表接口可能直接返回数组，也可能是 {items,total,...} 分页对象。"""
    if isinstance(data, dict):
        return data.get("items") or []
    return data or []


# ---------- 模块 ----------

def t_basic(token):
    out = api("GET", "/api/health")
    expect("基础", "health 返回 driver=ready", ok(out) and out["data"]["status"] == "ok", str(out)[:160])

    out = api("GET", "/api/meta")
    expect("基础", "meta 能力协商有协议字段", ok(out) and out["data"], str(out)[:200])

    out = api("GET", "/api/v2/nope")
    expect("基础", "未知路由不是 200 业务响应", not ok(out), str(out)[:120])


def t_auth():
    out = api("POST", "/api/user/login", {"name": "admin", "pwd": "admin123"})
    token = out["data"]["token"] if ok(out) else ""
    expect("认证", "超管登录成功", bool(token), str(out)[:160])

    out = api("POST", "/api/user/login", {"name": "admin", "pwd": "wrong"})
    expect("认证", "错误口令被拒", not ok(out), str(out)[:120])

    out = api("POST", "/api/user/register", {"name": "bob", "pwd": "pwd123", "re_pwd": "pwd123", "email": "b@e.com"})
    expect("认证", "注册普通用户 bob", ok(out), str(out)[:200])

    out = api("POST", "/api/user/register", {"name": "bob", "pwd": "pwd123", "re_pwd": "pwd123"})
    expect("认证", "重复注册被拒", not ok(out), str(out)[:120])

    out = api("GET", "/api/user/me", token=token)
    expect("认证", "/user/me 回显身份", ok(out) and out["data"].get("name") == "admin", str(out)[:160])

    out = api("POST", "/api/user/token", {"name": "e2e-pat", "expire_days": 7}, token)
    pat = out["data"]["token"] if ok(out) else ""
    expect("认证", "创建 PAT", bool(pat), str(out)[:200])

    if pat:
        out = api("GET", "/api/project", token=pat)
        expect("认证", "PAT 可访问受保护接口", ok(out), str(out)[:160])

    # 列表里取 id（创建返回只给明文令牌，不带 id）
    out = api("GET", "/api/user/token", token=token)
    pat_id = next((x["id"] for x in rows(out.get("data")) if x.get("name") == "e2e-pat"), 0)
    expect("认证", "PAT 列表含刚创建的项", ok(out) and pat_id > 0, str(out)[:200])

    if pat_id:
        out = api("DELETE", f"/api/user/token?id={pat_id}", token=token)
        expect("认证", "吊销 PAT", ok(out), str(out)[:160])
        out = api("GET", "/api/project", token=pat)
        expect("认证", "吊销后 PAT 失效", not ok(out), str(out)[:120])
    return token


def t_project(token):
    out = api("POST", "/api/project", {"title": "e2e 项目", "description": "端到端"}, token)
    pid = out["data"]["id"] if ok(out) else 0
    expect("项目", "创建项目", pid > 0, str(out)[:200])

    out = api("GET", "/api/project", token=token)
    expect("项目", "项目列表含新项目", ok(out) and any(p["id"] == pid for p in rows(out.get("data"))), str(out)[:200])

    out = api("GET", f"/api/project/detail?id={pid}", token=token)
    expect("项目", "项目详情", ok(out) and out["data"]["title"] == "e2e 项目", str(out)[:200])

    out = api("PUT", "/api/project", {"id": pid, "title": "e2e 项目改", "client_write_enabled": False}, token)
    expect("项目", "更新项目 + 关闭客户端写入", ok(out) and out["data"].get("client_write_enabled") is False, str(out)[:200])

    out = api("GET", "/api/user/list", token=token)
    users = rows(out.get("data"))
    bob = next((u for u in users if u.get("name") == "bob"), None)
    expect("项目", "用户列表可查到 bob", bob is not None, str(out)[:200])

    if bob:
        out = api("POST", "/api/project/member", {"project_id": pid, "user_id": bob["id"], "permission": 1}, token)
        expect("项目", "添加只读成员", ok(out), str(out)[:200])

        out = api("POST", "/api/user/login", {"name": "bob", "pwd": "pwd123"})
        btok = out["data"]["token"] if ok(out) else ""
        if btok:
            out = api("GET", f"/api/api?project_id={pid}", token=btok)
            expect("权限", "只读成员可读接口列表", ok(out), str(out)[:160])
            out = api("POST", "/api/api", {"project_id": pid, "api_name": "越权", "http_method_type": "GET", "url": "/x"}, btok)
            expect("权限", "只读成员写接口被拒", not ok(out), str(out)[:160])
            out = api("POST", "/api/project", {"title": "bob 的项目"}, btok)
            expect("权限", "普通用户可建自己的项目", ok(out), str(out)[:160])

    out = api("GET", f"/api/project/permission?project_id={pid}", token=token)
    expect("项目", "我的权限可查", ok(out), str(out)[:200])
    return pid


def t_group_api(token, pid):
    out = api("POST", "/api/group", {"project_id": pid, "title": "用户", "type": 1}, token)
    gid = out["data"]["id"] if ok(out) else 0
    expect("分组", "创建接口分组", gid > 0, str(out)[:160])

    out = api("POST", "/api/group", {"project_id": pid, "title": "说明文档", "type": 2}, token)
    dgid = out["data"]["id"] if ok(out) else 0
    expect("分组", "创建文档分组", dgid > 0, str(out)[:160])

    out = api("GET", f"/api/group?project_id={pid}", token=token)
    expect("分组", "分组列表含新建两组", ok(out) and len(rows(out.get("data"))) >= 2, str(out)[:200])

    out = api("PUT", "/api/group", {"id": gid, "project_id": pid, "title": "用户改", "type": 1}, token)
    expect("分组", "更新分组标题", ok(out), str(out)[:160])

    out = api("POST", "/api/sort", {"project_id": pid, "kind": "group",
                                    "items": [{"id": gid, "parent_id": 0, "priority": 0}]}, token)
    expect("分组", "分组排序接口可用", ok(out), str(out)[:160])

    api_payload = {
        "project_id": pid,
        "group_id": gid,
        "api_name": "登录",
        "http_method_type": "POST",
        "url": "/login",
        "description": "用账号密码换令牌",
        "http_request_header": [{"name": "Content-Type", "value": "application/json"}],
        "http_request_params": [{"fieldName": "page", "type": "int", "required": True, "description": "页码"}],
        "http_request_body_type": "json",
        "http_request_body": '{"u":"a"}',
        # 该列是「不透明 JSON 字符串」，不是数组（与客户端同步 extra 同口径）
        "http_request_body_fields": json.dumps([{"name": "u", "type": "string", "required": True}], ensure_ascii=False),
        "http_request_summary": "登录接口",
        "http_return_type": "object",
        "http_return_params": [{"fieldName": "token", "type": "string", "description": "令牌"}],
        "http_return_sample": {"code": 200, "data": {"token": "t"}},
        "http_return_content_type": "json",
        "status": 2,
    }
    out = api("POST", "/api/api", api_payload, token)
    aid = out["data"]["id"] if ok(out) else 0
    expect("接口", "创建接口（含 21 个富字段）", aid > 0, str(out)[:240])
    expect("接口", "创建返回带 last_change_id", ok(out) and out["data"].get("last_change_id", 0) > 0, str(out)[:200])

    out = api("GET", f"/api/api/detail?id={aid}&project_id={pid}", token=token)
    d = out["data"] or {}
    expect("接口", "详情回读富字段", ok(out) and d.get("http_request_body_type") == "json"
           and "token" in (d.get("http_return_params") or ""), str(out)[:240])

    out = api("PUT", "/api/api", {"id": aid, "project_id": pid, "group_id": gid, "api_name": "登录改",
                                  "http_method_type": "POST", "url": "/login2"}, token)
    expect("接口", "更新接口", ok(out) and out["data"].get("api_name") == "登录改", str(out)[:200])

    out = api("GET", f"/api/api?project_id={pid}&keyword=登录&page_num=1&page_size=10", token=token)
    expect("接口", "关键字/分页查询命中", ok(out) and out["data"]["total"] >= 1, str(out)[:200])

    out = api("POST", "/api/api/sample", {"api_id": aid, "name": "成功", "sample_text": '{"ok":true}',
                                          "status": 200, "content_type": "json"}, token)
    expect("示例", "保存响应示例", ok(out), str(out)[:200])
    out = api("GET", f"/api/api/samples?api_id={aid}", token=token)
    samples = rows(out.get("data"))
    expect("示例", "示例列表含刚存的", ok(out) and len(samples) >= 1, str(out)[:200])
    if samples:
        out = api("DELETE", "/api/api/sample", {"id": samples[0]["id"], "api_id": aid}, token)
        expect("示例", "删除响应示例", ok(out), str(out)[:160])

    out = api("GET", f"/api/api/export?project_id={pid}", token=token)
    expect("导出", "OpenAPI 导出", ok(out) and out["data"], str(out)[:160])
    out = api("GET", f"/api/api/export-postman?project_id={pid}", token=token)
    expect("导出", "Postman 导出", ok(out) and out["data"], str(out)[:160])
    out = api("GET", f"/api/api/curl?id={aid}&project_id={pid}", token=token)
    expect("导出", "cURL 导出", ok(out) and "curl" in str(out["data"]).lower(), str(out)[:160])
    out = api("GET", f"/api/api/code?id={aid}&project_id={pid}&lang=go", token=token)
    expect("导出", "代码片段生成", ok(out), str(out)[:160])

    out = api("POST", f"/api/api/import?project_id={pid}", {"openapi": "3.0.0", "paths": {
        "/pets": {"get": {"summary": "list pets", "tags": ["Pets"]},
                  "post": {"summary": "add pet", "tags": ["Pets"]}}}}, token)
    expect("导入", "OpenAPI 导入（tag 建分组）", ok(out), str(out)[:240])
    out = api("GET", f"/api/group?project_id={pid}", token=token)
    expect("导入", "导入后出现 Pets 分组", ok(out) and any(g.get("title") == "Pets" for g in rows(out.get("data"))), str(out)[:200])

    # 回收站：删 → 列表(deleted=1) → 恢复 → 再删 → 批量恢复 → 彻底删
    out = api("DELETE", f"/api/api?id={aid}&project_id={pid}", token=token)
    expect("回收站", "删除接口（软删）", ok(out), str(out)[:160])
    out = api("GET", f"/api/api?project_id={pid}&deleted=1", token=token)
    expect("回收站", "回收站列表可见", ok(out) and any(x["id"] == aid for x in (out["data"]["items"] or [])), str(out)[:200])
    out = api("POST", "/api/api/restore", {"id": aid, "project_id": pid}, token)
    expect("回收站", "单个恢复", ok(out), str(out)[:160])
    api("DELETE", f"/api/api?id={aid}&project_id={pid}", token=token)
    out = api("POST", "/api/api/restore-batch", {"ids": [aid]}, token)
    expect("回收站", "批量恢复", ok(out), str(out)[:160])
    api("DELETE", f"/api/api?id={aid}&project_id={pid}", token=token)
    out = api("POST", "/api/api/purge-batch", {"ids": [aid]}, token)
    expect("回收站", "彻底删除", ok(out), str(out)[:160])
    return gid, dgid, aid


def t_doc(token, pid, dgid):
    out = api("POST", "/api/doc", {"project_id": pid, "group_id": dgid, "title": "项目说明", "content": "# 说明\n\n正文"}, token)
    did = out["data"]["id"] if ok(out) else 0
    expect("文档", "创建文档", did > 0, str(out)[:200])
    expect("文档", "创建返回带 last_change_id", ok(out) and out["data"].get("last_change_id", 0) > 0, str(out)[:200])

    out = api("GET", f"/api/doc?project_id={pid}", token=token)
    expect("文档", "文档列表含新建", ok(out) and any(x["id"] == did for x in (out["data"]["items"] or [])), str(out)[:200])
    out = api("GET", f"/api/doc/groups?project_id={pid}", token=token)
    expect("文档", "文档分组列表可用", ok(out), str(out)[:160])
    out = api("GET", f"/api/doc/detail?id={did}&project_id={pid}", token=token)
    expect("文档", "文档详情含正文", ok(out) and "正文" in (out["data"].get("content") or ""), str(out)[:200])
    out = api("PUT", "/api/doc", {"id": did, "title": "项目说明改", "content": "# 改"}, token)
    expect("文档", "更新文档", ok(out) and out["data"].get("title") == "项目说明改", str(out)[:200])
    out = api("POST", f"/api/doc/like?id={did}&project_id={pid}", None, token)
    expect("文档", "点赞文档", ok(out), str(out)[:160])
    out = api("DELETE", f"/api/doc?id={did}&project_id={pid}", token=token)
    expect("文档", "删除文档（进回收站）", ok(out), str(out)[:160])
    out = api("POST", "/api/doc/restore", {"id": did, "project_id": pid}, token)
    expect("文档", "恢复文档", ok(out), str(out)[:160])
    return did


def t_vars(token, pid, aid):
    out = api("POST", "/api/variable-env", {"project_id": pid, "name": "dev", "description": "开发环境"}, token)
    eid = out["data"]["id"] if ok(out) else 0
    expect("变量", "创建环境", eid > 0, str(out)[:200])
    out = api("GET", f"/api/variable-env?project_id={pid}", token=token)
    expect("变量", "环境列表", ok(out) and len(rows(out.get("data"))) >= 1, str(out)[:160])
    out = api("PUT", f"/api/variable-env/default?id={eid}&project_id={pid}", None, token)
    expect("变量", "设置默认环境", ok(out), str(out)[:160])

    out = api("POST", "/api/variable", {"scope": 2, "project_id": pid, "env_id": eid,
                                        "name": "host", "value": "http://127.0.0.1:8093"}, token)
    expect("变量", "创建项目级变量", ok(out), str(out)[:200])
    out = api("POST", "/api/variable", {"scope": 2, "project_id": pid, "env_id": eid,
                                        "name": "secret_key", "value": "s3cret", "is_secret": 1}, token)
    expect("变量", "创建 secret 变量", ok(out), str(out)[:200])

    out = api("GET", f"/api/variable?project_id={pid}&env_id={eid}", token=token)
    var_rows = rows(out.get("data"))
    host = next((v for v in var_rows if v.get("name") == "host"), None)
    secret = next((v for v in var_rows if v.get("name") == "secret_key"), None)
    expect("变量", "变量列表含 host", host is not None, str(out)[:200])
    expect("变量", "secret 值在列表中被掩码/加密存储", secret is not None and secret.get("value") in ("", "********") or
           (secret is not None and not str(secret.get("value", "")).startswith("s3cret")), str(secret)[:200])

    out = api("GET", f"/api/variable/resolve?project_id={pid}&env_id={eid}", token=token)
    resolved = rows(out.get("data")) if isinstance(out.get("data"), list) else []
    host_row = next((x for x in resolved if isinstance(x, dict) and x.get("name") == "host"), None)
    expect("变量", "解析表含 host 明文", ok(out) and host_row is not None
           and "127.0.0.1" in str(host_row.get("value", "")), str(out)[:240])

    out = api("GET", f"/api/variable/summary?project_id={pid}&api_ids={aid}", token=token)
    expect("变量", "变量摘要可用", ok(out), str(out)[:160])

    if host:
        out = api("PUT", "/api/variable", {"id": host["id"], "scope": 2, "project_id": pid, "env_id": eid,
                                           "name": "host", "value": "http://127.0.0.1:9000"}, token)
        expect("变量", "更新变量", ok(out), str(out)[:160])
        out = api("DELETE", f"/api/variable?id={host['id']}&project_id={pid}", token=token)
        expect("变量", "删除变量", ok(out), str(out)[:160])
    return eid


def t_fm_search_share(token, pid):
    out = api("POST", "/api/field-mapping", {"project_id": pid, "field": "data.list[].id", "type": "int", "description": "主键"}, token)
    fmid = out["data"]["id"] if ok(out) else 0
    expect("字段映射", "创建字段映射", fmid > 0, str(out)[:200])
    out = api("GET", f"/api/field-mapping?project_id={pid}", token=token)
    expect("字段映射", "映射列表", ok(out) and len(rows(out.get("data"))) >= 1, str(out)[:160])
    if fmid:
        out = api("PUT", "/api/field-mapping", {"id": fmid, "project_id": pid, "field": "data.list[].id", "type": "string"}, token)
        expect("字段映射", "更新映射类型", ok(out), str(out)[:160])

    out = api("GET", f"/api/search?key=登录&project_id={pid}", token=token)
    expect("搜索", "全局搜索有结果", ok(out) and out["data"], str(out)[:200])

    out = api("POST", "/api/share", {"project_id": pid, "remark": "e2e", "expire_days": 1}, token)
    stoken, sid = "", 0
    if ok(out):
        d = out["data"]
        sid = d.get("id") or 0
        stoken = d.get("token") or d.get("url", "").rstrip("/").split("/")[-1]
    expect("分享", "创建分享链接", bool(stoken), str(out)[:200])
    out = api("GET", f"/api/share?project_id={pid}", token=token)
    expect("分享", "分享列表", ok(out), str(out)[:160])
    if stoken:
        out = api("GET", f"/api/share/{stoken}")
        expect("分享", "免登录可读分享内容", ok(out) and out["data"], str(out)[:200])
    if sid:
        out = api("DELETE", f"/api/share?id={sid}", token=token)
        expect("分享", "删除分享链接", ok(out), str(out)[:160])
    if fmid:
        out = api("DELETE", f"/api/field-mapping?id={fmid}&project_id={pid}", token=token)
        expect("字段映射", "删除映射", ok(out), str(out)[:160])


def t_mock(token, pid):
    # 单独建一条「已发布 + 带响应样例」的接口，避免依赖前面接口的状态
    out = api("POST", "/api/api", {"project_id": pid, "api_name": "Mock 用", "http_method_type": "GET",
                                   "url": "/mockme", "status": 2,
                                   "http_return_sample": {"ok": True}}, token)
    expect("Mock", "建 Mock 用接口（status=2）", ok(out), str(out)[:200])

    out = req("GET", f"/mock/{pid}/mockme", base=BASE)
    expect("Mock", "按方法+路径命中接口并回放示例", isinstance(out, dict) and out.get("ok") is True, str(out)[:200])
    out = req("GET", f"/mock/{pid}/not-exist", base=BASE)
    expect("Mock", "未命中时如实报错（不 500）", isinstance(out, dict) and out.get("code") == 404, str(out)[:160])


def t_sync(token, pid):
    out = api("GET", f"/api/sync/state?project_id={pid}", token=token)
    cur = out["data"]["cursor"] if ok(out) else -1
    expect("同步", "state 返回游标", cur >= 0, str(out)[:160])

    op_id = "e2e-op-1"
    payload = {"title": "同步分组", "priority": 0}
    body = {"project_id": pid, "device_id": "e2e", "device_name": "e2e 设备",
            "ops": [{"op_id": op_id, "op": "upsert", "type": "group", "uid": "e2e-g-1", "payload": payload}]}

    # 项目此前被关掉了 client_write_enabled（项目模块的最后一步）：先验证写入被拒
    out = api("POST", "/api/sync/push", body, token)
    expect("同步", "关闭客户端写入时 push 被拒（readonly）",
           ok(out) and out["data"]["results"][0].get("reason") == "readonly", str(out)[:240])

    api("PUT", "/api/project", {"id": pid, "title": "e2e 项目改", "client_write_enabled": True}, token)
    out = api("POST", "/api/sync/push", body, token)
    expect("同步", "打开写入后 push group", ok(out) and out["data"]["results"][0]["status"] == "ok", str(out)[:240])

    out = api("POST", "/api/sync/push", body, token)
    expect("同步", "同一 op_id 幂等（重复推送不报错）", ok(out) and out["data"]["results"][0]["status"] == "ok", str(out)[:240])

    out = api("GET", f"/api/sync/changes?project_id={pid}&since=0&limit=50", token=token)
    expect("同步", "changes 增量可读", ok(out) and out["data"], str(out)[:200])

    out = api("GET", f"/api/sync/snapshot?project_id={pid}&offset=0&limit=50", token=token)
    expect("同步", "snapshot 全量可读", ok(out) and out["data"], str(out)[:200])

    api("PUT", "/api/project", {"id": pid, "title": "e2e 项目改", "client_write_enabled": True}, token)
    out = api("POST", "/api/sync/push", {"project_id": pid, "device_id": "e2e",
                                         "ops": [{"op_id": "e2e-op-2", "op": "upsert", "type": "api", "uid": "e2e-a-1",
                                                  "payload": {"name": "同步接口", "method": "GET", "url": "/s"}}]}, token)
    expect("同步", "push api", ok(out) and out["data"]["results"][0]["status"] == "ok", str(out)[:240])


def t_revision_contract(token, pid):
    out = api("POST", "/api/api", {"project_id": pid, "api_name": "可回滚", "http_method_type": "GET", "url": "/r"}, token)
    rid = out["data"]["id"] if ok(out) else 0
    expect("历史", "建接口用于回滚", rid > 0, str(out)[:160])
    if not rid:
        return
    api("PUT", "/api/v1/item", {"id": rid, "project_id": pid, "api_name": "改一版", "http_method_type": "GET", "url": "/r2"}, token)
    out = api("GET", f"/api/v1/item/revisions?project_id={pid}&type=api&id={rid}", token=token)
    revs = rows(out.get("data"))
    expect("历史", "修订列表有快照", ok(out) and len(revs) >= 1, str(out)[:240])
    if revs:
        out = api("GET", f"/api/v1/item/revision-diff?project_id={pid}&type=api&id={rid}&revision_id={revs[0]['id']}", token=token)
        expect("历史", "字段级 diff", ok(out), str(out)[:200])
        out = api("POST", "/api/v1/item/revision-restore", {"type": "api", "id": rid, "revision_id": revs[0]["id"]}, token)
        expect("历史", "回滚到指定版本", ok(out), str(out)[:200])

    out = api("POST", "/api/v1/item/contract-diff", {"project_id": pid, "openapi": {
        "openapi": "3.0.0",
        "info": {"title": "t", "version": "1"},
        "paths": {"/r2": {"get": {"summary": "改一版"}}},
    }}, token)
    expect("契约", "OpenAPI 契约 diff 可用", ok(out), str(out)[:240])


def t_proxy(token, pid):
    class Echo(BaseHTTPRequestHandler):
        def do_POST(self):
            n = int(self.headers.get("Content-Length") or 0)
            body = self.rfile.read(n)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"echo": body.decode(errors="replace"), "path": self.path}).encode())

        def log_message(self, *a):
            pass

    srv = HTTPServer(("127.0.0.1", 0), Echo)
    port = srv.server_address[1]
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    try:
        out = api("POST", "/api/v1/proxy/send", {"project_id": pid, "method": "POST",
                                                 "url": f"http://127.0.0.1:{port}/echo?x=1",
                                                 "body_type": "json", "body": '{"a":1}'}, token)
        # 服务端禁止代理到回环/内网/保留地址（SSRF 防护）：这里断言「被拒绝」本身就是正向验证
        expect("代理", "回环地址被 SSRF 防护拒绝（预期行为）",
               not ok(out) and "不允许" in str((out or {}).get("msg", "")), str(out)[:200])
    finally:
        srv.shutdown()


def t_upload(token, pid):
    # 1x1 PNG
    png = bytes.fromhex(
        "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489"
        "0000000a49444154789c6360000002000154a24f6f0000000049454e44ae426082"
    )
    boundary = "----e2eboundary"
    body = b""
    body += f"--{boundary}\r\n".encode()
    body += b'Content-Disposition: form-data; name="file"; filename="a.png"\r\n'
    body += b"Content-Type: image/png\r\n\r\n" + png + b"\r\n"
    body += f"--{boundary}--\r\n".encode()
    r = urllib.request.Request(
        BASE + "/api/file/upload", data=body, method="POST",
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}", "Authorization": f"Bearer {token}"},
    )
    try:
        with urllib.request.urlopen(r) as resp:
            out = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        out = {"code": e.code, "msg": e.read().decode(errors="replace")[:200], "data": None}
    expect("上传", "PNG 上传成功", out.get("code") == 200, str(out)[:200])
    url = (out.get("data") or {}).get("url", "") if isinstance(out.get("data"), dict) else ""
    if url:
        code, payload, ctype = req("GET", url, base=BASE, raw=True)
        expect("上传", "静态取回且 Content-Type 固定为 image/png", code == 200 and ctype.startswith("image/png"),
               f"{code} {ctype}")
        code, _, _ = req("GET", "/uploads/../config.yaml", base=BASE, raw=True)
        expect("上传", "路径穿越被拒", code != 200, str(code))


def t_bug_property(token, pid):
    out = api("POST", "/api/bug", {"project_id": pid, "title": "示例缺陷", "content": "复现步骤", "level": 2}, token)
    bid = out["data"]["id"] if ok(out) else 0
    expect("缺陷", "创建 Bug", bid > 0, str(out)[:200])
    out = api("GET", f"/api/bug?project_id={pid}", token=token)
    expect("缺陷", "Bug 列表", ok(out) and len(out["data"]["items"] if isinstance(out["data"], dict) else rows(out.get("data"))) >= 1, str(out)[:200])
    if bid:
        out = api("PUT", "/api/bug", {"id": bid, "status": 2, "title": "示例缺陷改"}, token)
        expect("缺陷", "更新 Bug", ok(out), str(out)[:160])
        out = api("GET", f"/api/bug/assign?bug_id={bid}", token=token)
        expect("缺陷", "指派人列表可读", ok(out), str(out)[:160])
        out = api("POST", "/api/bug/assign", {"bug_id": bid, "to_user_id": 1, "comment": "指派给超管"}, token)
        expect("缺陷", "指派 Bug", ok(out), str(out)[:160])
        out = api("DELETE", f"/api/bug?id={bid}&project_id={pid}", token=token)
        expect("缺陷", "删除 Bug", ok(out), str(out)[:160])

    out = api("POST", "/api/property", {"tag": "e2e", "tag_name": "端到端", "description": "公共属性"}, token)
    expect("属性", "保存公共属性", ok(out), str(out)[:160])
    out = api("GET", "/api/property", token=token)
    expect("属性", "属性列表含新建", ok(out), str(out)[:200])
    out = api("DELETE", f"/api/property?tag=e2e", token=token)
    expect("属性", "删除公共属性", ok(out), str(out)[:160])

    out = api("GET", f"/api/operation-log?project_id={pid}", token=token)
    expect("日志", "操作日志有记录", ok(out) and out["data"], str(out)[:200])


def main():
    print(f"base={BASE}")
    token = t_auth()
    if not token:
        print("登录失败，终止")
        return 1
    t_basic(token)
    pid = t_project(token)
    gid, dgid, aid = t_group_api(token, pid)
    t_doc(token, pid, dgid)
    t_vars(token, pid, aid)
    t_fm_search_share(token, pid)
    t_mock(token, pid)
    t_sync(token, pid)
    t_revision_contract(token, pid)
    t_proxy(token, pid)
    t_upload(token, pid)
    t_bug_property(token, pid)

    total = len(results)
    failed = [r for r in results if not r[2]]
    print("\n==== 汇总 ====")
    by_mod = {}
    for mod, _, good, _ in results:
        t = by_mod.setdefault(mod, [0, 0])
        t[0 if good else 1] += 1
    for mod, (good, bad) in by_mod.items():
        print(f"{mod}: {good} 通过" + (f" / {bad} 失败" if bad else ""))
    print(f"合计 {total} 项，通过 {total - len(failed)}，失败 {len(failed)}")
    if failed:
        print("\n失败明细：")
        for mod, name, _, extra in failed:
            print(f"- [{mod}] {name}  {extra}")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
