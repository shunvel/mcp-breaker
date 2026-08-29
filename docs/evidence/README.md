# Test evidence captures

Terminal outputs and SVG renders committed for README test evidence.

| File | Description |
|------|-------------|
| `validate-terminal.txt` / `validate.svg` | `make validate` full run |
| `demo-terminal.txt` / `demo-echo.svg` | Echo breaker demo (Test Case A) |
| `echo-wrap-stderr.txt` / `echo-wrap.svg` | Wrap detector startup logs |
| `graph-wrap-stdout.txt` / `graph-loop.svg` | Graph ABAB block response |
| `semantic-test.txt` / `semantic-test.svg` | Test Case B unit test |
| `dashboard-terminal.txt` / `dashboard-tui.svg` | Live dashboard session |
| `ui/*.png` | Streamlit dev lab screenshots (`make evidence-ui`) |

Regenerate terminal evidence:

```bash
make evidence
```

Regenerate UI screenshots (requires `make dev-ui` on :8501):

```bash
make evidence-ui
```
