# Data Parsing — Broker Response Format

## Primary Format: Space-Delimited Sections

Broker responses are **plain text** with this structure:

```
date1,date2,...,dateN open1,open2,...,openN high1,high2,...,highN low1,low2,...,lowN close1,close2,...,closeN vol1,vol2,...,volN
```

### Parsing Steps (`parser.go` → `ParseBrokerResponse`)

```
Step 1: strings.Fields(responseText)   → split by whitespace → 6 sections
Step 2: strings.Split(section[i], ",") → split each section by comma
Step 3: Zip all arrays by index → build DailyData records
```

| `sections` index | Field   | Example values           |
|------------------|---------|--------------------------|
| `[0]`            | Dates   | `2024/01/02,2024/01/03`  |
| `[1]`            | Open    | `595,596`                |
| `[2]`            | High    | `600,598`                |
| `[3]`            | Low     | `590,591`                |
| `[4]`            | Close   | `598,597`                |
| `[5]`            | Volume  | `25000,18000`            |

**Minimum required sections:** 6 (including volume). Returns error if fewer.  
**Length check:** all arrays must have equal length, else error.

---

## Date Format Parsing

### AD Year: `parseDate()`

Format: `YYYY/MM/DD`

```go
parts := strings.Split("2024/01/02", "/")
// year=2024, month=1, day=2
time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
```

Valid range: year 1900–2100, month 1–12, day 1–31.

### ROC Year: `parseROCDate()`

Format: `YYY/MM/DD` (民國年)

```go
// "106/05/02" → year = 106 + 1911 = 2017, month=5, day=2
year += 1911
```

Used in the **tab-delimited fallback parser** (`ParseTabDelimited`).

---

## Numeric Value Cleaning: `cleanValue()`

Before `ParseFloat` / `ParseInt`, all values pass through:

| Remove | Reason              |
|--------|---------------------|
| `,`    | Thousand separator  |
| `$`    | Currency prefix     |
| `元`   | Chinese currency    |
| `TWD`  | Currency code       |
| spaces | Trim whitespace     |

**Skip rules:** if value is `""`, `"-"`, or `"N/A"` → skip the record (continue to next).

---

## Fallback Format: Tab-Delimited (`ParseTabDelimited`)

Used as backup when primary format fails.

```
日期\t開盤\t最高\t最低\t收盤\t成交量\n
```

- Date format: ROC year `YYY/MM/DD`
- Minimum 6 tab-separated columns per line
- Empty lines skipped

---

## Error Handling

| Error condition | Behavior |
|-----------------|----------|
| Empty response | return error |
| Fewer than 6 sections | return error |
| Mismatched array lengths | return error |
| Invalid date string | skip record (continue) |
| Non-numeric price/volume | skip record (continue) |
| All records invalid | return error "no valid data parsed" |
