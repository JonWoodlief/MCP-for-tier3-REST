#!/usr/bin/env python3

import asyncio
import json
import logging
from typing import Any, Dict, List, Optional, Sequence
from urllib.parse import urljoin

import httpx
from mcp.server import Server, NotificationOptions, request_ctx
from mcp.server.stdio import stdio_server
from mcp.server.models import InitializationOptions
from mcp.types import (
    Tool,
)

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("hateoas-mcp")

# Create server instance
server = Server("hateoas-mcp")

class HATEOASClient:
    def __init__(self, api_url: str = "http://localhost:9001"):
        self.api_url = api_url
        self.client = httpx.AsyncClient()
        self.cached_links: Dict[str, Dict[str, Any]] = {}
        self.cached_tools: List[Tool] = []
    
    async def discover_links(self) -> Dict[str, Any]:
        """Discover available HATEOAS links from the API root"""
        try:
            response = await self.client.get(f"{self.api_url}/account")
            if response.status_code == 200:
                data = response.json()
                old_links = self.cached_links.copy()
                self.cached_links = data.get("_links", {})
                self.cached_tools = self._generate_tools_from_links()
                
                # Send notification if tools have changed
                if old_links != self.cached_links:
                    await self._send_tool_refresh_notification()
                
                return data
            else:
                logger.error(f"Failed to discover links: {response.status_code}")
                return {}
        except Exception as e:
            logger.error(f"Error discovering links: {e}")
            return {}
    
    def _generate_tools_from_links(self) -> List[Tool]:
        """Generate tools list from cached links"""
        tools = []
        
        for link_name, link_data in self.cached_links.items():
            method = link_data.get("method", "GET").upper()
            rel = link_data.get("rel", link_name)
            
            # Create tool description based on method and relation
            description = f"Execute {rel} ({method})"
            
            # Create the tool
            try:
                tool = Tool(
                    name=link_name,
                    title=f"{rel.title()} Tool",
                    description=description,
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "amount": {
                                "type": "number",
                                "description": "Amount for the operation"
                            }
                        } if method in ["POST", "PUT", "PATCH"] else {},
                        "required": ["amount"] if method in ["POST", "PUT", "PATCH"] else []
                    }
                )
                
                tools.append(tool)
                
            except Exception as e:
                logger.error(f"Error creating tool {link_name}: {e}")
        
        return tools
    
    async def _send_tool_refresh_notification(self):
        """Send a tool list changed notification to the client"""
        try:
            # Get the current request context to access the session
            ctx = request_ctx.get()
            if ctx and ctx.session:
                await ctx.session.send_tool_list_changed()
                logger.info("Sent tool list changed notification")
        except Exception as e:
            logger.warning(f"Could not send tool refresh notification: {e}")
    
    async def execute_link(self, link_name: str, arguments: Dict[str, Any]) -> List[Dict[str, Any]]:
        """Execute a HATEOAS link with the given arguments"""
        # Refresh links to ensure we have the latest state
        await self.discover_links()
        
        # Find the link for this tool
        if link_name not in self.cached_links:
            return [{"type": "text", "text": f"Tool '{link_name}' is not currently available"}]
        
        link_data = self.cached_links[link_name]
        method = link_data.get("method", "GET").upper()
        href = link_data.get("href", "")
        
        # Convert relative URLs to absolute
        if href.startswith("/"):
            url = f"{self.api_url}{href}"
        elif href.startswith("http"):
            url = href
        else:
            url = urljoin(self.api_url, href)
        
        try:
            # Prepare request data
            request_data = {}
            if method in ["POST", "PUT", "PATCH"]:
                request_data = arguments
            
            headers = {"Content-Type": "application/json"}
            
            # Make the HTTP request
            response = None
            if method == "GET":
                response = await self.client.get(url, headers=headers)
            elif method == "POST":
                response = await self.client.post(url, json=request_data, headers=headers)
            elif method == "PUT":
                response = await self.client.put(url, json=request_data, headers=headers)
            elif method == "PATCH":
                response = await self.client.patch(url, json=request_data, headers=headers)
            elif method == "DELETE":
                response = await self.client.delete(url, headers=headers)
            
            # Handle response
            if response and response.status_code >= 200 and response.status_code < 300:
                try:
                    response_data = response.json()
                    
                    # Update cached links if the response contains them
                    if "_links" in response_data:
                        old_links = self.cached_links.copy()
                        self.cached_links = response_data["_links"]
                        self.cached_tools = self._generate_tools_from_links()
                        
                        # Send notification if tools have changed
                        if old_links != self.cached_links:
                            await self._send_tool_refresh_notification()
                    
                    # Format the response
                    formatted_response = json.dumps(response_data, indent=2)
                    
                    return [{"type": "text", "text": formatted_response}]
                except Exception:
                    return [{"type": "text", "text": f"Successfully executed {link_name} ({method})"}]
            else:
                return [{"type": "text", "text": f"Failed to execute {link_name}: HTTP {response.status_code if response else 'No response'}"}]
                
        except Exception as e:
            return [{"type": "text", "text": f"Error executing {link_name}: {str(e)}"}]
    
    async def cleanup(self):
        """Clean up resources"""
        await self.client.aclose()

# Global client instance
hateoas_client = HATEOASClient()

# Register handlers
@server.list_tools()
async def handle_list_tools() -> List[Tool]:
    """List available tools"""
    try:
        if not hateoas_client.cached_tools:
            await hateoas_client.discover_links()
        
        return hateoas_client.cached_tools
    except Exception as e:
        logger.error(f"Error in list_tools: {e}")
        return []

@server.call_tool()
async def handle_call_tool(name: str, arguments: Optional[dict] = None):
    """Execute a tool"""
    try:
        result = await hateoas_client.execute_link(name, arguments or {})
        return result
    except Exception as e:
        logger.error(f"Error calling tool {name}: {e}")
        return [{"type": "text", "text": f"Error executing {name}: {str(e)}"}]

async def main():
    """Main entry point"""
    # Enable tool change notifications
    notification_options = NotificationOptions(tools_changed=True)
    
    init_options = InitializationOptions(
        server_name="hateoas-mcp",
        server_version="1.0.0",
        capabilities=server.get_capabilities(
            notification_options=notification_options,
            experimental_capabilities={}
        )
    )
    
    try:
        async with stdio_server() as (read_stream, write_stream):
            await server.run(read_stream, write_stream, init_options)
    finally:
        await hateoas_client.cleanup()

if __name__ == "__main__":
    asyncio.run(main())