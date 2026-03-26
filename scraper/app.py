"""
stock-chips-pyramid scraper
抓取 norway.twsthr.info 的股票持股分佈資料，存入 PostgreSQL

API:
  POST /trigger          觸發全量爬取（以背景執行）
  POST /trigger-single   觸發單支股票爬取（body: {"symbol": "2330"}）
  GET  /status           回傳最新 job 狀態
  GET  /health           健康檢查
"""

import asyncio
import logging
import os
import re
import traceback
from datetime import datetime, date
from typing import Optional

import asyncpg
from aiohttp import web
from bs4 import BeautifulSoup
from dotenv import load_dotenv
from playwright.async_api import async_playwright, BrowserContext
from tenacity import retry, stop_after_attempt, wait_exponential

load_dotenv()

DATABASE_URL  = os.getenv("DATABASE_URL", "postgresql://postgres:postgres@postgres:5432/stockdb")
PORT          = int(os.getenv("SCRAPER_PORT", "5100"))
CONCURRENCY   = int(os.getenv("CONCURRENCY", "3"))
REQUEST_DELAY = float(os.getenv("REQUEST_DELAY", "2.0"))
HEADLESS      = os.getenv("HEADLESS", "true").lower() == "true"
# 預設方案："http"（aiohttp，輕量）或 "playwright"（瀏覽器模擬）
SCRAPE_METHOD = os.getenv("SCRAPE_METHOD", "http")

_HTTP_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    "Accept-Language": "zh-TW,zh;q=0.9",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
}

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

# ── DB helpers ───────────────────────────────────────────────────────────────

async def get_pool():
    return await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=5)

ENSURE_TABLES_SQL = """
CREATE TABLE IF NOT EXISTS chips_sync_jobs (
    id           BIGSERIAL PRIMARY KEY,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status       VARCHAR(20) NOT NULL DEFAULT 'running',
    total        INT NOT NULL DEFAULT 0,
    success      INT NOT NULL DEFAULT 0,
    fail         INT NOT NULL DEFAULT 0,
    message      TEXT
);

CREATE TABLE IF NOT EXISTS chips_holder_snapshots (
    id          BIGSERIAL PRIMARY KEY,
    job_id      BIGINT REFERENCES chips_sync_jobs(id),
    symbol      VARCHAR(10) NOT NULL,
    data_date   DATE NOT NULL,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (symbol, data_date)
);

CREATE TABLE IF NOT EXISTS chips_holder_distributions (
    id              BIGSERIAL PRIMARY KEY,
    snapshot_id     BIGINT NOT NULL REFERENCES chips_holder_snapshots(id) ON DELETE CASCADE,
    tier_rank       INT NOT NULL,
    range_label     VARCHAR(60) NOT NULL,
    holder_count    INT,
    holder_pct      NUMERIC(7,4),
    share_count     BIGINT,
    share_pct       NUMERIC(7,4),
    cum_holder_pct  NUMERIC(7,4),
    cum_share_pct   NUMERIC(7,4)
);
"""

async def ensure_tables(pool: asyncpg.Pool):
    async with pool.acquire() as conn:
        await conn.execute(ENSURE_TABLES_SQL)

async def recover_stale_jobs(pool: asyncpg.Pool):
    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE chips_sync_jobs
               SET status = 'failed',
                   completed_at = COALESCE(completed_at, NOW()),
                   message = COALESCE(NULLIF(message, ''), 'scraper restarted before job completed')
               WHERE status = 'running'"""
        )

async def create_job(pool: asyncpg.Pool, total: int) -> int:
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            """INSERT INTO chips_sync_jobs (started_at, status, total, success, fail, message)
               VALUES (NOW(), 'running', $1, 0, 0, '已啟動')
               RETURNING id""",
            total,
        )
        return row["id"]

async def update_job(pool: asyncpg.Pool, job_id: int, **kwargs):
    sets = ", ".join(f"{k} = ${i+2}" for i, k in enumerate(kwargs))
    vals = list(kwargs.values())
    async with pool.acquire() as conn:
        await conn.execute(
            f"UPDATE chips_sync_jobs SET {sets} WHERE id = $1",
            job_id, *vals,
        )

async def update_job_progress(pool: asyncpg.Pool, job_id: int, success: int, fail: int, message: str | None = None):
    fields = {
        "success": success,
        "fail": fail,
    }
    if message is not None:
        fields["message"] = message
    await update_job(pool, job_id, **fields)

async def finish_job(pool: asyncpg.Pool, job_id: int, success: int, fail: int):
    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE chips_sync_jobs
               SET status='completed', completed_at=NOW(), success=$2, fail=$3
               WHERE id=$1""",
            job_id, success, fail,
        )

async def fail_job(pool: asyncpg.Pool, job_id: int, msg: str):
    async with pool.acquire() as conn:
        await conn.execute(
            """UPDATE chips_sync_jobs
               SET status='failed', completed_at=NOW(), message=$2
               WHERE id=$1""",
            job_id, msg,
        )

async def get_symbols(pool: asyncpg.Pool) -> list[str]:
    """取出目前 stocks 資料表中的有效股票代碼。"""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT symbol FROM stocks WHERE symbol <> '' ORDER BY symbol"
        )
        return [r["symbol"] for r in rows]

async def save_snapshot(pool: asyncpg.Pool, job_id: int, symbol: str,
                        data_date: date, distributions: list[dict]) -> bool:
    """儲存快照與分佈資料，若同 symbol+data_date 已存在則略過"""
    async with pool.acquire() as conn:
        # UPSERT snapshot
        row = await conn.fetchrow(
            """INSERT INTO chips_holder_snapshots (job_id, symbol, data_date)
               VALUES ($1, $2, $3)
               ON CONFLICT (symbol, data_date) DO UPDATE SET job_id=$1
               RETURNING id""",
            job_id, symbol, data_date,
        )
        snap_id = row["id"]

        # Delete old distributions (in case of re-scrape)
        await conn.execute(
            "DELETE FROM chips_holder_distributions WHERE snapshot_id=$1", snap_id
        )

        # Insert distributions
        await conn.executemany(
            """INSERT INTO chips_holder_distributions
               (snapshot_id, tier_rank, range_label, holder_count, holder_pct,
                share_count, share_pct, cum_holder_pct, cum_share_pct)
               VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)""",
            [
                (snap_id, d["tier_rank"], d["range_label"],
                 d.get("holder_count"), d.get("holder_pct"),
                 d.get("share_count"), d.get("share_pct"),
                 d.get("cum_holder_pct"), d.get("cum_share_pct"))
                for d in distributions
            ],
        )
    return True

# ── Parser ───────────────────────────────────────────────────────────────────

def _parse_num(s: Optional[str]) -> Optional[float]:
    if not s:
        return None
    s = s.strip().replace(",", "").replace("%", "")
    if s in ("-", "N/A", "—", ""):
        return None
    try:
        return float(s)
    except ValueError:
        return None

def _roc_to_ad(s: str) -> Optional[date]:
    """民國年 YYY/MM/DD →西元"""
    m = re.match(r"(\d{2,3})/(\d{1,2})/(\d{1,2})", s.strip())
    if not m:
        return None
    y = int(m.group(1)) + 1911
    try:
        return date(y, int(m.group(2)), int(m.group(3)))
    except ValueError:
        return None

def _is_distribution_row(cells: list[str]) -> bool:
    if len(cells) < 6:
        return False
    label = cells[0].strip()
    # 持股區間格式：「1 ~ 999 股」「1,000 ~ 5,000 股」「1,000,001 股以上」
    return bool(re.search(r"(\d[\d,]*)\s*(~|以上|以下)", label))

def parse_page(html: str) -> tuple[Optional[date], list[dict]]:
    soup = BeautifulSoup(html, "html.parser")

    # 找資料日期
    data_date: Optional[date] = None
    for tag in soup.find_all(string=re.compile(r"\d{2,3}/\d{1,2}/\d{1,2}")):
        d = _roc_to_ad(tag.strip())
        if d:
            data_date = d
            break

    distributions = []
    for table in soup.find_all("table"):
        rows = table.find_all("tr")
        for rank, row in enumerate(rows, start=1):
            cells = [td.get_text(strip=True) for td in row.find_all(["td", "th"])]
            if not _is_distribution_row(cells):
                continue
            distributions.append({
                "tier_rank":     rank,
                "range_label":   cells[0].strip(),
                "holder_count":  int(_parse_num(cells[1]) or 0) or None,
                "holder_pct":    _parse_num(cells[2]),
                "share_count":   int(_parse_num(cells[3]) or 0) or None,
                "share_pct":     _parse_num(cells[4]),
                "cum_holder_pct": _parse_num(cells[5]) if len(cells) > 5 else None,
                "cum_share_pct":  _parse_num(cells[6]) if len(cells) > 6 else None,
            })

    return data_date, distributions

# ── Scraper: 方案 B — aiohttp（輕量 HTTP，不需瀏覽器）────────────────────────

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=8))
async def fetch_page_http(session: "aiohttp.ClientSession", symbol: str) -> str:
    from aiohttp import ClientSession
    url = f"https://norway.twsthr.info/StockHolders.aspx?stock={symbol}"
    async with session.get(url, headers=_HTTP_HEADERS) as resp:
        resp.raise_for_status()
        return await resp.text(encoding="utf-8", errors="replace")

async def scrape_symbol_http(session: "aiohttp.ClientSession", pool: asyncpg.Pool,
                              job_id: int, symbol: str, sem: asyncio.Semaphore) -> bool:
    async with sem:
        try:
            html = await fetch_page_http(session, symbol)
            data_date, distributions = parse_page(html)
            if not data_date or not distributions:
                log.warning(f"[{symbol}] [HTTP] 無資料")
                return False
            await save_snapshot(pool, job_id, symbol, data_date, distributions)
            log.info(f"[{symbol}] ✓ [HTTP] {data_date} {len(distributions)} 筆")
            await asyncio.sleep(REQUEST_DELAY)
            return True
        except Exception as e:
            log.error(f"[{symbol}] ✗ [HTTP] {e}")
            return False

async def _run_http_job(pool: asyncpg.Pool, job_id: int, symbols: list[str]):
    import aiohttp
    sem = asyncio.Semaphore(CONCURRENCY)
    success = fail = 0
    connector = aiohttp.TCPConnector(limit=CONCURRENCY + 5)
    timeout = aiohttp.ClientTimeout(total=30)
    async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
        async def scrape_one(symbol: str) -> tuple[str, bool]:
            return symbol, await scrape_symbol_http(session, pool, job_id, symbol, sem)

        tasks = [scrape_one(s) for s in symbols]
        for coro in asyncio.as_completed(tasks):
            symbol, result = await coro
            if result:
                success += 1
            else:
                fail += 1
            processed = success + fail
            await update_job_progress(pool, job_id, success, fail,
                                      f"處理中 {processed}/{len(symbols)}：{symbol}")
    return success, fail

# ── Scraper: 方案 C — Playwright（瀏覽器模擬）──────────────────────────────

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=8))
async def fetch_page_playwright(context: BrowserContext, symbol: str) -> str:
    url = f"https://norway.twsthr.info/StockHolders.aspx?stock={symbol}"
    page = await context.new_page()
    try:
        await page.goto(url, wait_until="networkidle", timeout=30_000)
        await page.wait_for_timeout(800)
        return await page.content()
    finally:
        await page.close()

async def scrape_symbol_playwright(context: BrowserContext, pool: asyncpg.Pool,
                                    job_id: int, symbol: str, sem: asyncio.Semaphore) -> bool:
    async with sem:
        try:
            html = await fetch_page_playwright(context, symbol)
            data_date, distributions = parse_page(html)
            if not data_date or not distributions:
                log.warning(f"[{symbol}] [PW] 無資料")
                return False
            await save_snapshot(pool, job_id, symbol, data_date, distributions)
            log.info(f"[{symbol}] ✓ [PW] {data_date} {len(distributions)} 筆")
            await asyncio.sleep(REQUEST_DELAY)
            return True
        except Exception as e:
            log.error(f"[{symbol}] ✗ [PW] {e}")
            return False

async def _run_playwright_job(pool: asyncpg.Pool, job_id: int, symbols: list[str]):
    sem = asyncio.Semaphore(CONCURRENCY)
    success = fail = 0
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(headless=HEADLESS)
        context = await browser.new_context(
            locale="zh-TW",
            extra_http_headers={"Accept-Language": "zh-TW,zh;q=0.9"},
        )
        try:
            async def scrape_one(symbol: str) -> tuple[str, bool]:
                return symbol, await scrape_symbol_playwright(context, pool, job_id, symbol, sem)

            tasks = [scrape_one(s) for s in symbols]
            for coro in asyncio.as_completed(tasks):
                symbol, result = await coro
                if result:
                    success += 1
                else:
                    fail += 1
                processed = success + fail
                await update_job_progress(pool, job_id, success, fail,
                                          f"處理中 {processed}/{len(symbols)}：{symbol}")
        finally:
            await context.close()
            await browser.close()
    return success, fail

# ── 統一入口 ─────────────────────────────────────────────────────────────────

async def run_scrape_job(pool: asyncpg.Pool, symbols: list[str], method: Optional[str] = None):
    """執行籌碼爬取作業。method: 'http'（預設） 或 'playwright'"""
    if not symbols:
        log.info("沒有要爬取的股票")
        return

    effective_method = method or SCRAPE_METHOD
    job_id = await create_job(pool, len(symbols))
    log.info(f"Job {job_id} 開始 method={effective_method}，共 {len(symbols)} 支股票")

    try:
        if effective_method == "playwright":
            success, fail = await _run_playwright_job(pool, job_id, symbols)
        else:
            success, fail = await _run_http_job(pool, job_id, symbols)

        await finish_job(pool, job_id, success, fail)
        await update_job(pool, job_id, message=f"完成：成功 {success}，失敗 {fail}")
        log.info(f"Job {job_id} 完成 method={effective_method} success={success} fail={fail}")
    except Exception as e:
        msg = f"job failed: {e}"
        log.error(msg)
        log.error(traceback.format_exc())
        await fail_job(pool, job_id, msg)

# ── HTTP server ───────────────────────────────────────────────────────────────

_pool: Optional[asyncpg.Pool] = None
_running_task: Optional[asyncio.Task] = None

async def get_db_pool() -> asyncpg.Pool:
    global _pool
    if _pool is None:
        _pool = await get_pool()
        await ensure_tables(_pool)
        await recover_stale_jobs(_pool)
    return _pool

async def handle_trigger(request: web.Request) -> web.Response:
    global _running_task
    if _running_task and not _running_task.done():
        return web.json_response({"ok": False, "msg": "已有爬取任務執行中"}, status=409)

    pool = await get_db_pool()
    body = {}
    try:
        body = await request.json()
    except Exception:
        pass

    symbol = body.get("symbol")
    method = body.get("method", SCRAPE_METHOD)  # "http" | "playwright"
    if symbol:
        symbols = [symbol]
    else:
        symbols = await get_symbols(pool)

    if not symbols:
        return web.json_response({"ok": False, "msg": "沒有股票需要爬取（請先同步股票清單）"}, status=400)

    _running_task = asyncio.create_task(run_scrape_job(pool, symbols, method))
    return web.json_response({"ok": True, "total": len(symbols), "method": method})

async def handle_status(request: web.Request) -> web.Response:
    pool = await get_db_pool()
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            """SELECT id, started_at, completed_at, status, total, success, fail, message
               FROM chips_sync_jobs ORDER BY id DESC LIMIT 1"""
        )
    if not row:
        return web.json_response({"status": "never"})

    return web.json_response({
        "id":           row["id"],
        "status":       row["status"],
        "started_at":   row["started_at"].isoformat() if row["started_at"] else None,
        "completed_at": row["completed_at"].isoformat() if row["completed_at"] else None,
        "total":        row["total"],
        "success":      row["success"],
        "fail":         row["fail"],
        "message":      row["message"],
    })

async def handle_health(request: web.Request) -> web.Response:
    return web.json_response({"status": "ok"})

async def init_app() -> web.Application:
    app = web.Application()
    app.router.add_post("/trigger",        handle_trigger)
    app.router.add_post("/trigger-single", handle_trigger)
    app.router.add_get("/status",          handle_status)
    app.router.add_get("/health",          handle_health)
    return app

if __name__ == "__main__":
    web.run_app(init_app(), host="0.0.0.0", port=PORT)
