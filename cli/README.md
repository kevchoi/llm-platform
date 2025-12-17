## llm-platform CLI (focus checks)

Captures your screen(s) + window list on macOS, sends them to an LLM, and prints a quick “are you on-task?” check.

### Requirements

- macOS (uses ScreenCaptureKit via PyObjC)
- Python 3.13+
- `ANTHROPIC_API_KEY` set in your environment
- Screen Recording permission enabled for your terminal/IDE:
  System Settings → Privacy & Security → Screen Recording

### Run

```bash
python main.py
```

You’ll be prompted for a goal; the tool will run periodic checks for 25 minutes, then prompt for a new goal.


Next:

- Hybrid search (dense + sparse), reranking, eval harness.