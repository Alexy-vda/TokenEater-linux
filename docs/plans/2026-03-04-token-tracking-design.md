# Local Token Tracking Design

## Goal
Track local token usage (from ~/.claude/projects/) aligned to the Anthropic API 5h window,
store locally in JSON, expose via D-Bus for the widget. Include API usage data so a future
server has everything without calling Anthropic.

## Snapshot Format
File: `~/.local/share/tokeneater/token-usage.json`

```json
{
  "hostname": "onefem05",
  "snapshots": [
    {
      "timestamp": "2026-03-04T16:00:00Z",
      "window": {
        "startsAt": "2026-03-04T12:00:00Z",
        "resetsAt": "2026-03-04T17:00:00Z"
      },
      "apiUsage": {
        "fiveHourUtilization": 22,
        "sevenDayUtilization": 7,
        "sevenDaySonnetUtilization": 2
      },
      "localTokens": {
        "inputTokens": 45230,
        "outputTokens": 12840,
        "cacheCreationTokens": 8500,
        "cacheReadTokens": 120000,
        "totalTokens": 186570
      }
    }
  ]
}
```

## Components

### tracker.go
- Receives `resetsAt` from the API five_hour bucket
- Computes window start: `resetsAt - 5h`
- Scans `~/.claude/projects/*/*.jsonl` for files modified in last 6h
- Parses lines with `"usage"` block, filters by timestamp in [startsAt, resetsAt]
- Sums: input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens

### storage.go
- Reads/writes `~/.local/share/tokeneater/token-usage.json`
- Appends snapshot each fetch cycle (every 5 min)
- Prunes snapshots older than 7 days
- Includes hostname (from os.Hostname)

### Integration
- main.go: after successful API fetch, call tracker then storage
- DaemonState gets new `tokenUsage` field
- D-Bus emits updated state including token counts
- Widget displays "Tokens (5h): 186.6k" in popup

## Future
- Server sync: external agent reads token-usage.json and POSTs to monitoring server
- hostname identifies the machine
