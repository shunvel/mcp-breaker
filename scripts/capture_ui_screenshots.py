#!/usr/bin/env python3
"""Capture Streamlit dev lab screenshots for README (requires server on :8501)."""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "evidence" / "ui"
URL = "http://localhost:8501"


def select_scenario(page, query: str) -> None:
    combo = page.get_by_role("combobox", name="Scenario")
    combo.click()
    combo.fill(query)
    page.wait_for_timeout(300)
    combo.press("Enter")
    page.wait_for_timeout(800)


def scroll_bottom(page) -> None:
    page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
    page.wait_for_timeout(500)


def clip_bottom(page, path: Path) -> None:
    page.locator("div.phase-title", has_text="Live dashboard").scroll_into_view_if_needed()
    page.wait_for_timeout(400)
    page.screenshot(path=str(path))


def main() -> int:
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("Install playwright: python3 -m pip install playwright && python3 -m playwright install chromium")
        return 1

    OUT.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page(viewport={"width": 1280, "height": 900})

        page.goto(URL, wait_until="networkidle", timeout=60000)
        page.wait_for_timeout(2000)
        page.screenshot(path=str(OUT / "01-home.png"), full_page=True)
        print("  captured 01-home.png")

        expand = page.get_by_test_id("stExpandSidebarButton")
        if expand.count():
            expand.click()
            page.wait_for_timeout(500)
        page.locator('[data-testid="stSidebar"]').screenshot(path=str(OUT / "04-settings-sidebar.png"))
        print("  captured 04-settings-sidebar.png")
        collapse = page.get_by_test_id("stSidebarCollapseButton")
        if collapse.count():
            collapse.click()
            page.wait_for_timeout(300)

        page.get_by_role("button", name="▶ Start proxy").click()
        page.wait_for_timeout(2500)
        page.screenshot(path=str(OUT / "02-proxy-running.png"), full_page=True)
        print("  captured 02-proxy-running.png")

        page.get_by_role("button", name="▶ Run test: Echo loop").click()
        page.wait_for_timeout(3500)
        scroll_bottom(page)
        page.screenshot(path=str(OUT / "03-echo-results.png"), full_page=True)
        print("  captured 03-echo-results.png")

        select_scenario(page, "Semantic")
        page.get_by_role("button", name="▶ Run test: Semantic stagnation").click()
        page.wait_for_timeout(4000)
        scroll_bottom(page)
        page.screenshot(path=str(OUT / "05-semantic-results.png"), full_page=True)
        print("  captured 05-semantic-results.png")

        clip_bottom(page, OUT / "06-dashboard.png")
        print("  captured 06-dashboard.png")

        browser.close()

    print(f"\nScreenshots saved to {OUT}/")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
