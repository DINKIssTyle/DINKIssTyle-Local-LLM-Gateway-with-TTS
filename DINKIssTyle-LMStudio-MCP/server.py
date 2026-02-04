# Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

from mcp.server.fastmcp import FastMCP
from playwright.async_api import async_playwright
import asyncio
import logging
import nest_asyncio
from dotenv import load_dotenv

load_dotenv()
nest_asyncio.apply()

# Initialize FastMCP server
mcp = FastMCP("browser-mcp")

# Global variables for Playwright
playwright = None
browser = None


async def ensure_browser():
    """Ensure Playwright browser is initialized."""
    global playwright, browser
    if playwright is None:
        playwright = await async_playwright().start()
    if browser is None:
        browser = await playwright.chromium.launch(headless=True)
    return browser

from duckduckgo_search import DDGS

@mcp.tool()
async def search_web(query: str) -> str:
    """
    Search the web using DuckDuckGo and return the results.
    Args:
        query: The search query string.
    Returns:
        A formatted string containing search results.
    """
    try:
        results = DDGS().text(query, max_results=5)
        if not results:
            return "No results found."
            
        formatted_results = []
        for i, res in enumerate(results):
            title = res.get('title', 'No title')
            link = res.get('href', 'No link')
            snippet = res.get('body', 'No description')
            formatted_results.append(f"[{i+1}] {title}\nLink: {link}\nSummary: {snippet}\n")
            
        return "\n".join(formatted_results)
    except Exception as e:
        return f"Error performing search: {str(e)}"

@mcp.tool()
async def read_website(url: str) -> str:
    """
    Read the content of a website associated with a URL.
    Args:
        url: The URL of the website to read.
    Returns:
        The text content of the website.
    """
    try:
        browser = await ensure_browser()
    except Exception as e:
        return f"Error initializing browser: {str(e)}"

    page = await browser.new_page()
    try:
        await page.goto(url, timeout=30000) # 30 seconds timeout
        
        # Get the main content text
        # Try to get specific content areas or fallback to body
        content = await page.evaluate("() => document.body.innerText")
        
        # Basic cleanup: limit length to avoid context overflow (approx 10k chars)
        max_length = 10000
        if len(content) > max_length:
            content = content[:max_length] + "\n... (Content truncated)"
            
        title = await page.title()
        return f"Title: {title}\nURL: {url}\n\nContent:\n{content}"
        
    except Exception as e:
        return f"Error reading website: {str(e)}"
    finally:
        await page.close()

if __name__ == "__main__":
    mcp.run()
