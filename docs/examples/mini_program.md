# Mini Program Development
> AI-Powered Development Environment

An AI-assisted mini program development IDE that demonstrates AEnvironment's capabilities for tool integration, deployment, and scaling. This IDE enables developers to create web applications through natural language conversations with an AI agent.

## Features

- **AI Agent**: Multi-turn conversation powered by OpenAI API with automatic retry logic for rate limits
- **Virtual File System (VFS)**: Persistent file system for managing mini program files
- **MCP Tools**: Model Context Protocol tools for file operations and code execution
- **Live Preview**: Real-time preview of generated applications in an iframe
- **Responsive Design**: Automatically ensures generated content fits the preview window
- **Version Tracking**: Displays version numbers on generated pages for update verification

## Architecture

The project is organized into three main directories:

```
mini-program/
├── agent/              # Agent server (FastAPI + OpenAI integration)
│   └── agent_server.py
├── environment/        # AEnvironment tools and configuration
│   ├── src/            # MCP tools (read_file, write_file, etc.)
│   ├── config.json     # Environment configuration
│   ├── Dockerfile      # Container build configuration
│   └── requirements.txt
└── frontend/           # Web UI (HTML/CSS/JavaScript)
    ├── index.html
    ├── app.js
    └── style.css
```

### Component Overview

- **Frontend**: HTML/CSS/JS interface with split view (chat | live preview)
- **Agent Server**: FastAPI backend that integrates OpenAI API with MCP tools
- **MCP Server**: AEnvironment MCP server providing tools via Model Context Protocol
- **Tools**:
  - `read_file`: Read files from virtual file system
  - `write_file`: Write files to virtual file system
  - `list_files`: List all files in the VFS
  - `execute_python_code`: Execute Python code snippets for validation
  - `validate_html`: Validate HTML structure and syntax
  - `check_responsive_design`: Analyze CSS for responsive design compliance

## Prerequisites

1. **Python 3.8+** installed
2. **AEnvironment SDK** installed (`pip install aenvironment`)
3. **OpenAI API Key** (required for AI chat functionality)
4. **Node.js and npm** (for MCP Inspector, installed automatically by `aenv run`)

## Installation

1. **Install Python dependencies**:

```bash
pip install -r environment/requirements.txt
```

2. **Set environment variables**:

```bash
export OPENAI_API_KEY='your-api-key-here'
export OPENAI_MODEL='gpt-4'  # Optional, defaults to gpt-4
export OPENAI_BASE_URL='https://api.openai.com/v1'  # Optional, for custom endpoints
export AENV_ENV_NAME='mini-program@1.0.0'  # Optional, environment name
```

## Quick Start

### Step 1: Start the MCP Server (Environment Side)

Open a terminal and navigate to the `environment` directory:

```bash
cd environment
aenv run
```

This will:
- Start the AEnvironment MCP server
- Load tools from `environment/src/`
- Start the MCP Inspector (available at `http://localhost:6274`)

**Keep this terminal running.**

### Step 2: Start the Agent Server (Agent Side)

Open a **new terminal** and navigate to the `agent` directory:

```bash
cd agent
python agent_server.py
```

This will:
- Start the FastAPI server on port 8080
- Connect to the MCP server for tool access
- Enable the web interface

**Keep this terminal running.**

### Step 3: Open the Web Interface

Open your browser and navigate to:

```
http://localhost:8080
```

### Step 4: Start Building

1. Type your request in the chat input (e.g., "Create a simple counter mini program")
2. The AI agent will:
   - Analyze your request
   - Use tools to read existing files (if any)
   - Create or modify files as needed
   - Generate complete HTML/CSS/JavaScript applications
3. The Live Preview will automatically update to show your generated application
4. Click the "🔄 Refresh" button to manually refresh the preview

## Usage Examples

### Example 1: Create a Counter Application

**User**: "Create a simple counter mini program"

**Agent Actions**:
1. Uses `list_files` to check existing files
2. Creates `index.html` with HTML structure, CSS styling, and JavaScript logic
3. Uses `validate_html` to check for errors
4. Uses `check_responsive_design` to ensure responsive layout
5. Preview automatically updates

**Result**: A fully functional counter application appears in the Live Preview.

### Example 2: Modify Existing Application

**User**: "Add a reset button to the counter"

**Agent Actions**:
1. Uses `read_file` to read the existing `index.html`
2. Modifies the file to add a reset button
3. Increments the version number displayed on the page
4. Preview updates automatically

**Result**: The counter now includes a reset button.

### Example 3: Create a Game

**User**: "Create a Snake game"

**Agent Actions**:
1. Plans the game structure (HTML canvas, game logic, controls)
2. Creates `index.html` with complete game implementation
3. Ensures responsive design (scales to fit preview window)
4. Adds version number tracking
5. Preview shows the playable game

**Result**: A fully playable Snake game in the Live Preview.

## Workflow

1. **User Input**: Type a natural language request in the chat
2. **Agent Planning**: AI agent analyzes the request and plans the implementation
3. **Tool Execution**: Agent uses MCP tools to:
   - Check existing files (`list_files`)
   - Read files if needed (`read_file`)
   - Create/modify files (`write_file`)
   - Validate code (`validate_html`, `check_responsive_design`)
   - Execute Python code for calculations (`execute_python_code`)
4. **Automatic Preview**: System automatically renders the generated HTML in Live Preview
5. **Iteration**: User can provide feedback and continue refining the application

## Features in Detail

### Rate Limit Handling

The agent server includes automatic retry logic with exponential backoff for OpenAI API rate limit errors (429):
- **Max Retries**: 3 attempts
- **Initial Delay**: 1 second
- **Backoff Factor**: 2x (delays: 1s, 2s, 4s)
- **Max Delay**: 60 seconds

### Virtual File System

- **Persistent Storage**: Files are stored in `/tmp/code/` on disk
- **In-Memory Cache**: Fast access to frequently used files
- **Automatic Loading**: Existing files are loaded on server startup
- **Clear Function**: Use `/api/clear` endpoint to reset the VFS

### Responsive Design Enforcement

The agent is instructed to:
- Use viewport-relative units (vw, vh, %, clamp)
- Ensure content fits without scrolling
- Scale canvas elements dynamically
- Use flexible layouts (flexbox, grid)

### Version Tracking

Every generated `index.html` includes a visible version number that increments with each modification, helping users verify that updates are working correctly.

## API Endpoints

### Agent Server Endpoints

- `GET /`: Serve the main HTML interface
- `POST /api/chat`: Send chat messages and receive AI responses
- `GET /api/files`: List all files in VFS
- `GET /api/files/{file_path}`: Get file content
- `POST /api/files/{file_path}`: Save file content
- `POST /api/clear`: Clear all files from VFS
- `GET /health`: Health check endpoint
- `WebSocket /ws`: Real-time communication (for future use)

## Configuration

### Environment Variables

- `OPENAI_API_KEY` (required): Your OpenAI API key
- `OPENAI_MODEL` (optional): Model to use (default: "gpt-4")
- `OPENAI_BASE_URL` (optional): Custom API endpoint URL
- `AENV_ENV_NAME` (optional): Environment name (default: "mini-program@1.0.0")

### Project Configuration

- `environment/config.json`: AEnvironment project metadata
- `environment/Dockerfile`: Container build configuration
- `environment/requirements.txt`: Python dependencies

## Troubleshooting

### MCP Server Not Starting

- Ensure you're in the `environment` directory when running `aenv run`
- Check that `environment/src/` contains the tool files
- Verify AEnvironment SDK is installed: `pip list | grep aenv`

### Agent Server Connection Issues

- Ensure the MCP server is running first
- Check that `AENV_ENV_NAME` matches the environment name
- Verify OpenAI API key is set correctly

### Static Files Not Loading

- Ensure you're running `agent_server.py` from the `agent/` directory
- Check that `frontend/` directory exists at the project root
- Verify FastAPI is serving static files (check server logs)

### Rate Limit Errors

- The system automatically retries with exponential backoff
- If errors persist, check your OpenAI API quota
- Consider using a different model or reducing request frequency

## Testing

The project includes basic functionality tests. Run tests using pytest or your preferred testing framework.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (User)                        │
│              http://localhost:8080                      │
└────────────────────┬────────────────────────────────────┘
                     │ HTTP/WebSocket
┌────────────────────▼────────────────────────────────────┐
│              Agent Server (FastAPI)                      │
│              Port: 8080                                  │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  OpenAI API Client                               │   │
│  │  - Chat Completions                              │   │
│  │  - Tool Calling                                  │   │
│  │  - Retry Logic (429 handling)                    │   │
│  └──────────────┬───────────────────────────────────┘   │
│                 │ HTTP API                              │
│  ┌──────────────▼───────────────────────────────────┐   │
│  │  MCP Client                                      │   │
│  │  - Tool Discovery                                │   │
│  │  - Tool Execution                                │   │
│  └──────────────┬───────────────────────────────────┘   │
└─────────────────┼───────────────────────────────────────┘
                  │ MCP Protocol (HTTP)
┌─────────────────▼───────────────────────────────────────┐
│         AEnvironment MCP Server                          │
│         Port: 8081                                       │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  MCP Tools (from environment/src/)               │   │
│  │  - read_file                                     │   │
│  │  - write_file                                    │   │
│  │  - list_files                                    │   │
│  │  - execute_python_code                           │   │
│  │  - validate_html                                 │   │
│  │  - check_responsive_design                       │   │
│  └──────────────┬───────────────────────────────────┘   │
│                 │                                        │
│  ┌──────────────▼───────────────────────────────────┐   │
│  │  Virtual File System (VFS)                       │   │
│  │  Storage: /tmp/code/                             │   │
│  │  - Persistent on disk                            │   │
│  │  - In-memory cache                               │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

## Development

### Project Structure

- `agent/`: Agent server code (FastAPI application)
- `environment/src/`: MCP tools implementation
- `frontend/`: Web UI files
- `test_*.py`: Test scripts for verification

### Adding New Tools

1. Create a new tool function in `environment/src/mini_program_tools.py`
2. Use the `@register_tool` decorator
3. Restart the MCP server (`aenv run`)
4. The tool will be automatically available to the agent

### Modifying the Frontend

- Edit files in `frontend/` directory
- Changes take effect immediately (no rebuild needed)
- Refresh the browser to see updates

## License

Copyright 2025. Licensed under the Apache License, Version 2.0.

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review server logs for error messages
3. Verify all prerequisites are installed
4. Ensure environment variables are set correctly
