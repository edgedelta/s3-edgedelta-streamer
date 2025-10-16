# Log Format Guide

The streamer can normalize almost any structured or semi-structured log file by configuring patterns in `config.yaml`. This guide collects the full reference for defining and testing formats.

## Configuration Structure

```yaml
processing:
  log_formats:
    - name: "format_name"
      filename_pattern: "*.log.gz"
      timestamp_regex: "(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}Z)"
      timestamp_format: "2006-01-02T15:04:05Z"
      content_type: "text/plain"
      skip_header_lines: 0
      field_separator: " "
  default_format: "auto"
```

- **`filename_pattern`** – Glob matched before downloading the file.
- **`timestamp_regex`** – Must include a **single capture group** containing the timestamp.
- **`timestamp_format`** – Any Go time layout, or the keywords `unix` / `unix_ms`.
- **`content_type`** – HTTP `Content-Type` header when the batch is sent.
- **`skip_header_lines`** – How many lines to drop from the top of the file.
- **`field_separator`** – Optional delimiter for CSV-style payloads.

## Supported Timestamp Formats

- `unix` – unix epoch seconds (`1705315200`)
- `unix_ms` – unix epoch milliseconds (`1705315200000`)
- Go time layouts – e.g. `2006-01-02T15:04:05Z`, `Jan _2 15:04:05`.

## Common Examples

### AWS CloudTrail (JSON)
```yaml
- name: "cloudtrail"
  filename_pattern: "*.json.gz"
  timestamp_regex: "(\\d{4}\\d{2}\\d{2}T\\d{2}\\d{2}\\d{2}Z)"
  timestamp_format: "20060102T150405Z"
  content_type: "application/x-ndjson"
```

### ELB / ALB Access Logs
```yaml
- name: "elb_access"
  filename_pattern: "*.log.gz"
  timestamp_regex: "(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{6}Z)"
  timestamp_format: "2006-01-02T15:04:05.000000Z"
  content_type: "text/plain"
  field_separator: " "
```

### CloudFront Access Logs
```yaml
- name: "cloudfront"
  filename_pattern: "*.gz"
  timestamp_regex: "(\\d{4}-\\d{2}-\\d{2}\\t\\d{2}:\\d{2}:\\d{2})"
  timestamp_format: "2006-01-02\t15:04:05"
  content_type: "text/plain"
  field_separator: "\t"
```

### Additional Recipes

| Format | Key Pattern |
| --- | --- |
| `vpc_flow` | `(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{6}Z)` |
| `s3_access` | `\\[(\\d{2}/[A-Za-z]{3}/\\d{4}:\\d{2}:\\d{2}:\\d{2} [+-]\\d{4})\\]` |
| `rds_mysql_slow` | `# Time: (\\d{6}\s+\\d{1,2}:\\d{2}:\\d{2})` |
| `rds_postgres` | `(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2} [A-Z]{3})` |
| `lambda` | `(\\d{4}/\\d{2}/\\d{2}/\\d{2}/\\d{4}-\\d{6})` |
| `app_json_iso` | `"timestamp":"([^"]+)"` |
| `syslog_rfc3164` | `([A-Za-z]{3}\\s+\\d{1,2} \\d{2}:\\d{2}:\\d{2})` |
| `apache_combined` | `\\[(\\d{2}/[A-Za-z]{3}/\\d{4}:\\d{2}:\\d{2}:\\d{2} [+-]\\d{4})\\]` |
| `nginx_access` | Same as Apache |
| `json_unix` | `"timestamp":(\\d+)` |
| `generic_csv` | `^(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2})` |
| `text_iso` | `(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(\\.\\d{3})?Z?)` |
| `kubernetes` | `"timestamp":"([^"]+)"` |

> **Tip:** Filename-derived timestamps are dramatically faster than scanning file contents. Prefer filename patterns whenever possible.

## Auto-Detection

Set `default_format: "auto"` to let the streamer choose the best match automatically:

1. Try filename patterns against the registry.
2. If ambiguous, sample content and run `DetectFromContent` hooks.
3. Fall back to the first defined format if nothing matches.

## Creating a New Format

1. Identify where the timestamp lives (filename vs content).
2. Write a regex with exactly one capture group containing the timestamp.
3. Choose the matching Go layout or `unix`/`unix_ms`.
4. Test with tools such as https://regex101.com/ or the Go playground.
5. Add the configuration to `processing.log_formats` and restart the service.

> **Warning:** Expensive regular expressions can increase S3 processing latency. Keep patterns specific and anchored when possible.
