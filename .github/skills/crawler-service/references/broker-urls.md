# Broker URLs & Stock List API

## Daily OHLCV Data — Broker URLs

Configured in `configs/config.yaml` under `crawler.broker_urls`.
BrokerManager iterates in order; **first success wins** (Failover strategy).

| Priority | Broker       | Base URL                                      |
|----------|--------------|-----------------------------------------------|
| 1        | 富邦         | `http://fubon-ebrokerdj.fbs.com.tw/`          |
| 2        | 玉山         | `http://justdata.moneydj.com/`                |
| 3        | 元大         | `http://jdata.yuanta.com.tw/`                 |
| 4        | MoneyDJ      | `http://moneydj.emega.com.tw/`                |
| 5        | 富邦基金     | `http://djfubonholdingfund.fbs.com.tw/`       |
| 6        | 玉山證券     | `https://sjmain.esunsec.com.tw/`              |
| 7        | 群益          | `http://kgieworld.moneydj.com/`              |
| 8        | 元富          | `http://newjust.masterlink.com.tw/`           |

**To add a broker:** append the URL to `broker_urls` list in `configs/config.yaml`. No code change needed.

---

## Stock List APIs

### 上市股票 — Taiwan Stock Exchange (TWSE)

```
GET https://www.twse.com.tw/exchangeReport/STOCK_DAY_ALL?response=json
```

**Response structure:**
```json
{
  "data": [
    ["2330", "台積電", ...],
    ["2317", "鴻海",   ...]
  ]
}
```

- `data[*][0]` → stock code
- `data[*][1]` → stock name
- Filter: code must be exactly **4 numeric digits**

### 上櫃股票 — Taipei Exchange (TPEX)

```
GET https://www.tpex.org.tw/web/stock/aftertrading/otc_quotes_no1430/stk_wn1430_result.php?l=zh-tw&d=<ROC_DATE>
```

Date format: ROC year `YYY/MM/DD` (e.g., `114/03/25` for 2025-03-25)

**Response structure:**
```json
{
  "aaData": [
    ["3711", "日月光投控", ...],
    ["5269", "祥碩",       ...]
  ]
}
```

- `aaData[*][0]` → stock code  (**Note: key is `aaData`, not `data`**)
- `aaData[*][1]` → stock name
- Filter: code must be exactly **4 numeric digits**

### Fallback Default Lists

When API fails, the service uses embedded defaults:

**TWSE defaults (20 stocks):** 2330 台積電, 2317 鴻海, 2412 中華電, 2881 富邦金, 2882 國泰金, 2303 聯電, 2308 台達電, 2454 聯發科, 2886 兆豐金, 2891 中信金, 2002 中鋼, 2301 光寶科, 2379 瑞昱, 2395 研華, 3008 大立光, 2357 華碩, 2409 友達, 2892 第一金, 3045 台灣大, 2327 國巨

**TPEX defaults (10 stocks):** 3711 日月光投控, 5269 祥碩, 6115 鎧勝-KY, 4904 遠傳, 5274 信驊, 6669 緯穎, 4938 和碩, 3231 緯創, 6505 台塑化, 5388 中磊
