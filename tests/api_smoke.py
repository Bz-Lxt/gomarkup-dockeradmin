"""DockerAdmin API 冒烟测试（Mock 模式，外部调用成本 ¥0）。

运行方式：
  - 宿主机：  BASE_URL=http://localhost:19217 pytest tests/api_smoke.py -v
  - Compose： docker compose --profile test run --rm tests

覆盖：健康检查 / 系统指标 / SSE 流 / 容器只读与幂等操作 / 告警规则 CRUD /
     校验错误信封 / 告警触发→Mock Webhook 全链路 / SPA 回退。
"""

import json
import os
import time
import uuid

import pytest
import requests

BASE_URL = os.environ.get("BASE_URL", "http://localhost:19217").rstrip("/")
# 告警规则里的 webhook 回跳地址：compose 内为 http://app:8080，宿主机直测时用 BASE_URL
WEBHOOK_URL = os.environ.get("WEBHOOK_URL", f"{BASE_URL}/api/mock/webhook")

TIMEOUT = 10
session = requests.Session()


def wait_for(predicate, timeout=20, interval=1.0, desc="condition"):
    """轮询直到 predicate 为真；超时则 fail 并带上下文。"""
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        last = predicate()
        if last:
            return last
        time.sleep(interval)
    pytest.fail(f"等待超时（{timeout}s）：{desc}；最后一次结果：{last!r}")


def api(method, path, **kwargs):
    kwargs.setdefault("timeout", TIMEOUT)
    return session.request(method, f"{BASE_URL}{path}", **kwargs)


# ---------------------------------------------------------------- 健康检查

def test_health():
    r = api("GET", "/api/health")
    assert r.status_code == 200, r.text
    body = r.json()["data"]
    assert body["status"] == "ok"
    assert body["docker"] in ("connected", "degraded")
    assert body["uptime_sec"] >= 0
    assert "version" in body


# ---------------------------------------------------------------- 系统指标

def _current_metrics():
    r = api("GET", "/api/metrics/current")
    return r if r.status_code == 200 else None


def test_metrics_current_shape():
    """采集器首帧需要至多一个采集周期，允许短暂等待。"""
    r = wait_for(_current_metrics, desc="首个指标快照就绪")
    snap = r.json()["data"]
    assert 0 <= snap["cpu"]["percent"] <= 100
    assert len(snap["cpu"]["per_core"]) >= 1
    mem = snap["mem"]
    assert mem["total"] > 0 and 0 <= mem["percent"] <= 100
    assert isinstance(snap["disk"], list) and isinstance(snap["net"], list)
    assert len(snap["load"]) == 3
    assert snap["procs"] >= 1
    # 时间戳可解析且带时区
    ts = snap["ts"]
    assert "T" in ts and ("+" in ts or ts.endswith("Z"))


def test_metrics_history_window():
    r = api("GET", "/api/metrics/history", params={"minutes": 5})
    assert r.status_code == 200, r.text
    data = r.json()["data"]
    assert isinstance(data, list) and len(data) >= 1
    assert all("ts" in s and "cpu" in s for s in data)


def test_metrics_history_param_validation():
    r = api("GET", "/api/metrics/history", params={"minutes": 0})
    assert r.status_code in (400, 422)
    err = r.json()["error"]
    assert err["code"] and err["message"]


def test_sse_metrics_stream():
    """SSE：10s 内应收到至少一帧 data（采集周期 2s）。"""
    with session.get(f"{BASE_URL}/api/stream/metrics", stream=True, timeout=15) as r:
        assert r.status_code == 200
        assert "text/event-stream" in r.headers.get("Content-Type", "")
        deadline = time.time() + 12
        for line in r.iter_lines(decode_unicode=True):
            assert time.time() < deadline, "12s 内未收到 SSE 数据帧"
            if line and line.startswith("data:"):
                frame = json.loads(line[5:].strip())
                assert "cpu" in frame and "mem" in frame
                return
        pytest.fail("SSE 流结束但未收到任何 data 帧")


# ---------------------------------------------------------------- 容器（只读 + 幂等）

def _docker_connected():
    return api("GET", "/api/health").json()["data"]["docker"] == "connected"


def test_containers_list_or_degraded():
    r = api("GET", "/api/containers")
    if _docker_connected():
        assert r.status_code == 200, r.text
        items = r.json()["data"]
        assert isinstance(items, list)
        # 应用自身容器应在列表中
        names = [c["name"] for c in items]
        assert any("dockeradmin" in n for n in names), f"未找到自身容器：{names}"
    else:
        assert r.status_code == 503
        assert r.json()["error"]["code"] == "docker_unavailable"


def test_container_detail_and_logs():
    if not _docker_connected():
        pytest.skip("Docker 降级模式，跳过容器详情测试")
    items = api("GET", "/api/containers").json()["data"]
    self_c = next(c for c in items if "dockeradmin" in c["name"])

    r = api("GET", f"/api/containers/{self_c['id']}")
    assert r.status_code == 200, r.text
    detail = r.json()["data"]
    assert detail["id"].startswith(self_c["id"][:12]) or self_c["id"].startswith(detail["id"][:12])
    assert detail["state"] == "running"
    assert "env_preview" in detail

    r = api("GET", f"/api/containers/{self_c['id']}/logs", params={"tail": 10})
    assert r.status_code == 200, r.text
    assert "lines" in r.json()["data"]


def test_container_start_idempotent_on_running():
    """对已运行的自身容器执行 start 是幂等 no-op（安全，不影响服务）。"""
    if not _docker_connected():
        pytest.skip("Docker 降级模式，跳过容器操作测试")
    items = api("GET", "/api/containers").json()["data"]
    self_c = next(c for c in items if "dockeradmin" in c["name"])
    r = api("POST", f"/api/containers/{self_c['id']}/start")
    assert r.status_code == 204, r.text


def test_container_action_not_found():
    if not _docker_connected():
        pytest.skip("Docker 降级模式，跳过容器操作测试")
    r = api("POST", "/api/containers/nonexistent123/stop")
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "not_found"


# ---------------------------------------------------------------- 告警规则 CRUD

def _rule_payload(**overrides):
    payload = {
        "name": f"smoke-{uuid.uuid4().hex[:8]}",
        "metric": "cpu_percent",
        "target": "",
        "op": ">",
        "threshold": 80,
        "duration_sec": 5,
        "cooldown_sec": 60,
        "enabled": True,
        "webhook_url": WEBHOOK_URL,
        "notify_recovery": True,
    }
    payload.update(overrides)
    return payload


def test_alert_rule_crud():
    # Create
    r = api("POST", "/api/alert-rules", json=_rule_payload())
    assert r.status_code == 201, r.text
    rule = r.json()["data"]
    assert rule["id"]
    assert r.headers.get("Location", "").endswith(rule["id"])

    # Read（列表包含）
    rules = api("GET", "/api/alert-rules").json()["data"]
    assert any(x["id"] == rule["id"] for x in rules)

    # Update
    updated = _rule_payload(name=rule["name"], threshold=90)
    r = api("PUT", f"/api/alert-rules/{rule['id']}", json=updated)
    assert r.status_code == 200, r.text
    assert r.json()["data"]["threshold"] == 90

    # Delete
    r = api("DELETE", f"/api/alert-rules/{rule['id']}")
    assert r.status_code == 204, r.text
    rules = api("GET", "/api/alert-rules").json()["data"]
    assert all(x["id"] != rule["id"] for x in rules)


def test_alert_rule_validation_error_envelope():
    r = api("POST", "/api/alert-rules", json=_rule_payload(threshold=150))
    assert r.status_code == 422, r.text
    err = r.json()["error"]
    assert err["code"] == "validation_error"
    assert any(d["field"] == "threshold" for d in err["details"])

    r = api("POST", "/api/alert-rules", json=_rule_payload(metric="container_cpu_percent", target=""))
    assert r.status_code == 422
    assert any(d["field"] == "target" for d in r.json()["error"]["details"])


# ---------------------------------------------------------------- 告警触发 → Mock Webhook 全链路

def test_alert_fires_and_webhook_received():
    """阈值 >= 0 必触发；验证 引擎判定→Webhook 推送→Mock 接收→事件记录 全链路。"""
    r = api("POST", "/api/alert-rules",
            json=_rule_payload(op=">=", threshold=0, duration_sec=0, cooldown_sec=0))
    assert r.status_code == 201, r.text
    rule_id = r.json()["data"]["id"]
    try:
        def _fired():
            events = api("GET", "/api/alert-events").json()["data"]
            return next((e for e in events
                         if e["rule_id"] == rule_id and e["kind"] == "fired"), None)

        event = wait_for(_fired, timeout=30, desc="告警触发事件出现")
        assert event["webhook_status"] == 200, f"Webhook 投递失败：{event}"

        def _receipt():
            receipts = api("GET", "/api/mock/webhook/receipts").json()["data"]
            for x in receipts:
                payload = json.loads(x["payload"])  # payload 为原始请求体字符串
                if payload.get("rule_id") == rule_id:
                    return payload
            return None

        payload = wait_for(_receipt, timeout=10, desc="Mock Webhook 接收记录")
        assert payload["kind"] == "fired"
        assert payload["metric"] == "cpu_percent"
    finally:
        api("DELETE", f"/api/alert-rules/{rule_id}")


# ---------------------------------------------------------------- Mock Webhook 基础

def test_mock_webhook_roundtrip():
    marker = uuid.uuid4().hex
    r = api("POST", "/api/mock/webhook", json={"marker": marker})
    assert r.status_code == 200, r.text
    assert r.json()["data"]["received"] is True

    receipts = api("GET", "/api/mock/webhook/receipts").json()["data"]
    assert any(json.loads(x["payload"]).get("marker") == marker for x in receipts)


# ---------------------------------------------------------------- 前端 SPA

def test_spa_fallback_and_index():
    r = api("GET", "/")
    assert r.status_code == 200
    assert "text/html" in r.headers.get("Content-Type", "")
    assert "<div id=\"app\">" in r.text or 'id="app"' in r.text

    r = api("GET", "/containers")  # 非 /api 路径回退 index.html
    assert r.status_code == 200
    assert "text/html" in r.headers.get("Content-Type", "")
