"""Offline engineering RAG. SQLite postings, exact hybrid cosine, no network.

Records remain immutable in the harness audit store. This derived index is
replaceable and refuses a different corpus digest or retrieval implementation.
Concepts are explicit vocabulary, not learned embeddings.
"""
import collections
import hashlib
import json
import math
import os
from pathlib import Path
import sqlite3
import sys
import tempfile

VERSION = "python-sqlite-cosine-v2"
CONCEPTS = [
    ["duplicate", "replay", "idempotency", "重复", "重放", "幂等"],
    ["quantity", "decimal", "precision", "数量", "小数", "精度"],
    ["speech", "audio", "microphone", "voice", "语音", "录音", "麦克风"],
    ["image", "photo", "vision", "图片", "照片", "视觉", "拍照"],
    ["menu", "recipe", "plan", "菜单", "菜谱", "配方"],
    ["timeout", "latency", "slow", "超时", "延迟", "缓慢"],
    ["permission", "authorization", "权限", "越权", "授权"],
    ["cache", "redis", "缓存"],
    ["database", "transaction", "数据库", "事务"],
    ["environment", "docker", "infrastructure", "环境", "容器"],
]


def lexical(text):
    words = "".join(c if c.isalpha() or c.isdecimal() else " " for c in text.lower()).split()
    counts = collections.Counter(words)
    for word in words:
        counts.update(word[i:i+2] for i in range(len(word)-1) if ord(word[i]) > 127)
    return counts


def vector(text):
    lex = lexical(text)
    norm = math.sqrt(sum(v*v for v in lex.values()))
    result = {"l:"+k: v/norm * math.sqrt(.55) for k, v in lex.items()} if norm else {}
    concepts = [str(i) for i, words in enumerate(CONCEPTS) if any(w in text.lower() for w in words)]
    if concepts:
        result.update({"c:"+k: math.sqrt(.45/len(concepts)) for k in concepts})
    return result


def active(records):
    current = {}
    for f in records:
        if f["id"] not in current or f["revision"] > current[f["id"]]["revision"]:
            current[f["id"]] = f
    live = [f for f in current.values() if f["status"] not in ("deleted", "obsolete")]
    replaced = {f.get("replaces") for f in live}
    return sorted((f for f in live if f["id"] not in replaced), key=lambda f: f["id"])


def build(path, records, digest):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temp = tempfile.mkstemp(prefix="rag-", suffix=".sqlite", dir=path.parent)
    os.close(fd)
    db = sqlite3.connect(temp)
    try:
        db.executescript("""
            CREATE TABLE meta (version TEXT, digest TEXT);
            CREATE TABLE docs (id INTEGER PRIMARY KEY, name TEXT UNIQUE, body TEXT);
            CREATE TABLE postings (term TEXT, doc INTEGER, weight REAL, PRIMARY KEY(term,doc)) WITHOUT ROWID;
            CREATE TABLE filters (kind TEXT, value TEXT, doc INTEGER, PRIMARY KEY(kind,value,doc)) WITHOUT ROWID;
        """)
        db.execute("INSERT INTO meta VALUES (?,?)", (VERSION, digest))
        for i, f in enumerate(active(records)):
            body = " ".join(f.get(k, "") for k in ("symptom", "conditions", "root_cause", "fix"))
            db.execute("INSERT INTO docs VALUES (?,?,?)", (i, f["id"], json.dumps(f, ensure_ascii=False)))
            db.executemany("INSERT INTO postings VALUES (?,?,?)", ((t, i, w) for t, w in vector(body).items()))
            fields = {"business": f.get("business") or [], "test": f.get("tests") or [],
                      "applicable_version": f.get("applicable_versions") or [],
                      "category": [f.get("category", "")], "error_code": [f.get("error_code", "")],
                      "verified_only": ["yes"] if f.get("verified") and f["status"] == "resolved" else []}
            db.executemany("INSERT INTO filters VALUES (?,?,?)",
                           ((k, v, i) for k, values in fields.items() for v in set(values)))
        db.commit()
        db.close()
        os.replace(temp, path)
    finally:
        db.close()
        if os.path.exists(temp):
            os.unlink(temp)
    return {"version": VERSION, "digest": digest}


class Index:
    def __init__(self, path, digest):
        self.db = sqlite3.connect(Path(path).resolve().as_uri()+"?mode=ro", uri=True)
        if self.db.execute("SELECT version,digest FROM meta").fetchone() != (VERSION, digest):
            self.close()
            raise ValueError("stale experience index; rebuild required")

    def close(self):
        self.db.close()

    def query(self, q):
        text = q.get("query", "")
        if not isinstance(text, str) or len(text) > 8000:
            raise ValueError("invalid query")
        if any(marker in (text+" "+q.get("disposition", "")).lower() for marker in
               ("bearer ", "api_key", "password=", "authorization:", "-----begin", "sk-")):
            raise ValueError("redact sensitive query")
        v = vector(text)
        if not v:
            return []
        clauses, params = [], []
        for kind in ("business", "category", "error_code", "applicable_version", "test", "verified_only"):
            value = "yes" if kind == "verified_only" and q.get(kind) else q.get(kind)
            if value:
                clauses.append("SELECT doc FROM filters WHERE kind=? AND value=?")
                params.extend((kind, value))
        # Exact score: no candidate truncation or approximate search. Filtering
        # is applied before ranking and tie-breaking is stable by public ID.
        eligible = " AND p.doc IN ("+" INTERSECT ".join(clauses)+")" if clauses else ""
        sql = "WITH q(term,weight) AS (VALUES "+",".join("(?,?)" for _ in v)+") "
        sql += "SELECT d.body,SUM(p.weight*q.weight) AS score FROM q JOIN postings p ON p.term=q.term JOIN docs d ON d.id=p.doc WHERE 1=1"
        sql += eligible + " GROUP BY p.doc HAVING score>=0.15 ORDER BY score DESC,d.name LIMIT 5"
        rows = self.db.execute(sql, [x for item in v.items() for x in item]+params).fetchall()
        result = []
        for body, score in rows:
            f = json.loads(body)
            verified = bool(f.get("verified") and f["status"] == "resolved")
            result.append({"id": f["id"], "score": score, "verified": verified,
                           "historical_cause": f.get("root_cause", "") if verified else "Unverified hypothesis: "+f.get("root_cause", ""),
                           "historical_fix": f.get("fix", "") if verified else "Requires diagnosis; no verified repair",
                           "scope": f.get("scope", ""), "citations": f.get("evidence") or [],
                           "version_mismatch": f.get("code_version", "") != q.get("code_version", ""),
                           "conflicts": f.get("conflicts") or []})
        return result


def run(request):
    if request["action"] == "evidence":
        return [{"id": r["id"], "fix": r["historical_fix"], "scope": r["scope"],
                 "version_mismatch": r["version_mismatch"], "verified": True, "conflicts": []}
                for r in request["results"] if r.get("verified") is True and not r.get("conflicts")]
    if request["action"] == "build":
        return build(request["path"], request["records"], request["digest"])
    if request["action"] == "query":
        index = Index(request["path"], request["digest"])
        try:
            return index.query(request["query"])
        finally:
            index.close()
    raise ValueError("unknown RAG action")


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8", errors="strict")
    try:
        raw = sys.stdin.buffer.read(256*1024*1024+1)
        if len(raw) > 256*1024*1024:
            raise ValueError("input limit")
        print(json.dumps(run(json.loads(raw)), ensure_ascii=False, allow_nan=False))
    except Exception:
        # Never emit private knowledge, paths or inherited environment.
        print('{"error":"Python RAG failed; check index, digest and request"}')
        sys.exit(1)
