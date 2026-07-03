# Lab 31: Capstone Project — Backend Development Assistant

## Objectives

- Build an end-to-end assistant that takes a plain-text requirements description.
- Generate a design document, API code, tests, and a review report as artifacts.
- Persist all artifacts to a local `output/` directory.
- Use environment variables for credentials and produce graceful errors.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

You can customize the requirements by editing `requirements.txt` or by setting the `REQUIREMENTS` environment variable:

```bash
REQUIREMENTS="Build a todo list API with user auth." python main.py
```

## Expected Output

The assistant writes the following files under `output/`:

- `design_doc.md` — System overview, endpoints, data model, assumptions.
- `api_code.py` — Runnable FastAPI application.
- `test_api.py` — pytest test suite.
- `review_report.md` — Review checklist, risks, and recommendations.

Console output shows the artifact paths and a completion summary.
