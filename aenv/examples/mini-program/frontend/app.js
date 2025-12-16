// Mini Program IDE Frontend JavaScript

const API_BASE = '/api';
let conversationHistory = [];

// DOM Elements
const chatMessages = document.getElementById('chatMessages');
const chatInput = document.getElementById('chatInput');
const sendBtn = document.getElementById('sendBtn');
const previewFrame = document.getElementById('previewFrame');
const chatStatus = document.getElementById('chatStatus');
const refreshPreviewBtn = document.getElementById('refreshPreviewBtn');
// Initialize
document.addEventListener('DOMContentLoaded', () => {
    addTerminalLog('System initialized', 'info');
    loadInitialPreview();
});

// Send message
sendBtn.addEventListener('click', sendMessage);
chatInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
    }
});

async function sendMessage() {
    const message = chatInput.value.trim();
    if (!message) return;

    // Add user message to chat
    addMessage('user', message);
    chatInput.value = '';
    chatInput.disabled = true;
    sendBtn.disabled = true;
    chatStatus.textContent = 'Thinking...';
    chatStatus.style.color = '#dcdcaa';

    try {
        const response = await fetch(`${API_BASE}/chat`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                message: message,
                stream: false,
            }),
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        
        // Add assistant response
        addMessage('assistant', data.content, data.tool_calls, data.tool_results);

        // Show iteration info if available
        if (data.iterations) {
            addTerminalLog(`Completed ${data.iterations} iteration(s)`, 'info');
        }

        // If tools were called, update preview
        if (data.tool_results && data.tool_results.length > 0) {
            addTerminalLog(`Executed ${data.tool_results.length} tool call(s)`, 'info');
            
            let hasFileWrite = false;
            for (const toolResult of data.tool_results) {
                if (toolResult.tool_name === 'write_file') {
                    const filePath = toolResult.result?.result?.file_path || toolResult.result?.file_path || 'unknown';
                    addTerminalLog(`✓ File written: ${filePath}`, 'success');
                    hasFileWrite = true;
                } else if (toolResult.tool_name === 'read_file') {
                    const filePath = toolResult.result?.result?.file_path || toolResult.result?.file_path || 'unknown';
                    addTerminalLog(`📖 File read: ${filePath}`, 'info');
                } else if (toolResult.tool_name === 'list_files') {
                    const files = toolResult.result?.result?.files || toolResult.result?.files || [];
                    addTerminalLog(`📁 Listed ${files.length} file(s)`, 'info');
                } else if (toolResult.tool_name === 'execute_python_code') {
                    const result = toolResult.result?.result || toolResult.result;
                    if (result && result.output) {
                        addTerminalLog(`🐍 Python output: ${result.output}`, 'info');
                    }
                    if (result && result.error) {
                        addTerminalLog(`🐍 Python error: ${result.error}`, 'error');
                    }
                }
            }
            
            // Refresh preview after file writes (with delay to ensure files are saved)
            if (hasFileWrite) {
                setTimeout(() => refreshPreview(), 1000);
            }
        }

        chatStatus.textContent = 'Ready';
        chatStatus.style.color = '#858585';
    } catch (error) {
        console.error('Error sending message:', error);
        addMessage('assistant', `Error: ${error.message}`);
        addTerminalLog(`Error: ${error.message}`, 'error');
        chatStatus.textContent = 'Error';
        chatStatus.style.color = '#f48771';
    } finally {
        chatInput.disabled = false;
        sendBtn.disabled = false;
        chatInput.focus();
    }
}

function addMessage(role, content, toolCalls = null, toolResults = null) {
    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${role}`;

    const contentDiv = document.createElement('div');
    contentDiv.className = 'message-content';

    // Add content - process redacted_reasoning tags
    if (content) {
        // Check if content contains XML-like tags
        if (content.includes('<think>') || 
            content.includes('<think>') || 
            content.includes('<reasoning>') ||
            content.includes('</think>') ||
            content.includes('</think>') ||
            content.includes('</reasoning>')) {
            
            // Process HTML-like tags - normalize to <think> tags
            let processedContent = content
                .replace(/<think>/gi, '<think>')
                .replace(/<\/redacted_reasoning>/gi, '</think>')
                .replace(/<reasoning>/gi, '<think>')
                .replace(/<\/reasoning>/gi, '</think>');
            
            // Split by lines and process
            const lines = processedContent.split('\n');
            let inThinkTag = false;
            let htmlContent = '';
            
            lines.forEach(line => {
                const trimmed = line.trim();
                
                if (trimmed.includes('<think>')) {
                    inThinkTag = true;
                    htmlContent += line + '\n';
                } else if (trimmed.includes('</think>')) {
                    inThinkTag = false;
                    htmlContent += line + '\n';
                } else if (inThinkTag) {
                    // Inside think tag, keep as is
                    htmlContent += line + '\n';
                } else {
                    // Regular content, wrap in <p>
                    if (trimmed) {
                        htmlContent += `<p>${escapeHtml(line)}</p>\n`;
                    }
                }
            });
            
            contentDiv.innerHTML = htmlContent;
        } else {
            // Regular text content
            const p = document.createElement('p');
            p.textContent = content;
            contentDiv.appendChild(p);
        }
    }

    // Add tool calls - elegant display
    if (toolCalls && toolCalls.length > 0) {
        // Group tool calls if multiple
        if (toolCalls.length > 1) {
            const toolGroup = document.createElement('div');
            toolGroup.className = 'tool-group';
            const header = document.createElement('div');
            header.className = 'tool-group-header';
            header.textContent = `${toolCalls.length} Tool Calls`;
            toolGroup.appendChild(header);
            
            toolCalls.forEach(toolCall => {
                const toolDiv = createToolCallElement(toolCall);
                toolGroup.appendChild(toolDiv);
            });
            contentDiv.appendChild(toolGroup);
        } else {
            const toolDiv = createToolCallElement(toolCalls[0]);
            contentDiv.appendChild(toolDiv);
        }
    }

    // Add tool results - elegant display
    if (toolResults && toolResults.length > 0) {
        // Group tool results if multiple
        if (toolResults.length > 1) {
            const resultGroup = document.createElement('div');
            resultGroup.className = 'tool-group';
            const header = document.createElement('div');
            header.className = 'tool-group-header';
            header.textContent = `${toolResults.length} Results`;
            resultGroup.appendChild(header);
            
            toolResults.forEach(toolResult => {
                const resultDiv = createToolResultElement(toolResult);
                resultGroup.appendChild(resultDiv);
            });
            contentDiv.appendChild(resultGroup);
        } else {
            const resultDiv = createToolResultElement(toolResults[0]);
            contentDiv.appendChild(resultDiv);
        }
    }

    messageDiv.appendChild(contentDiv);
    chatMessages.appendChild(messageDiv);
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

// Helper function to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Create tool call element
function createToolCallElement(toolCall) {
    const toolDiv = document.createElement('div');
    toolDiv.className = 'tool-call';
    
    // Extract key arguments for display
    const args = toolCall.arguments || {};
    let argSummary = '';
    
    if (Object.keys(args).length > 0) {
        const entries = Object.entries(args).slice(0, 2);
        argSummary = entries
            .map(([key, val]) => {
                let valStr = '';
                if (typeof val === 'string') {
                    valStr = val.length > 25 ? val.substring(0, 25) + '...' : val;
                } else if (typeof val === 'object' && val !== null) {
                    valStr = JSON.stringify(val).substring(0, 25);
                } else {
                    valStr = String(val).substring(0, 25);
                }
                return `<span style="color: #858585;">${escapeHtml(key)}</span>: <span style="color: #d4d4d4;">${escapeHtml(valStr)}</span>`;
            })
            .join(' • ');
    }
    
    toolDiv.innerHTML = `
        <span class="tool-name">${escapeHtml(toolCall.name)}</span>
        ${argSummary ? `<span class="tool-args">${argSummary}</span>` : ''}
    `;
    return toolDiv;
}

// Create tool result element
function createToolResultElement(toolResult) {
    const resultDiv = document.createElement('div');
    resultDiv.className = 'tool-result';
    
    // Simplify result display with better formatting
    const result = toolResult.result || {};
    let resultSummary = '';
    let resultType = 'info';
    
    if (result.success !== undefined) {
        resultSummary = result.success 
            ? 'Operation completed successfully' 
            : `Failed: ${result.error || 'Unknown error'}`;
        resultType = result.success ? 'success' : 'error';
    } else if (result.content) {
        const content = typeof result.content === 'string' ? result.content : JSON.stringify(result.content);
        resultSummary = content.length > 80 ? content.substring(0, 80) + '...' : content;
        resultType = 'content';
    } else if (result.files) {
        resultSummary = `Found ${result.files.length} file(s): ${result.files.slice(0, 3).join(', ')}${result.files.length > 3 ? '...' : ''}`;
        resultType = 'files';
    } else if (result.file_path) {
        resultSummary = `File: ${result.file_path}${result.size ? ` (${result.size} bytes)` : ''}`;
        resultType = 'file';
    } else {
        const jsonStr = JSON.stringify(result);
        resultSummary = jsonStr.length > 80 ? jsonStr.substring(0, 80) + '...' : jsonStr;
    }
    
    resultDiv.innerHTML = `
        <div class="tool-name">${escapeHtml(toolResult.tool_name)}</div>
        <div class="tool-summary">${escapeHtml(resultSummary)}</div>
    `;
    return resultDiv;
}

function addTerminalLog(message, type = 'info') {
    const timestamp = new Date().toLocaleTimeString();
    console.log(`[${timestamp}] [${type.toUpperCase()}] ${message}`);
}

async function loadInitialPreview() {
    try {
        // Try to get files and render initial preview
        const response = await fetch(`${API_BASE}/files`);
        if (response.ok) {
            const data = await response.json();
            if (data.success && data.result && data.result.files) {
                // Check if we have an index.html or app.js
                const files = data.result.files;
                if (files.includes('index.html')) {
                    renderPreview('index.html');
                }
            }
        }
    } catch (error) {
        console.error('Error loading initial preview:', error);
    }
}

async function renderPreview(filePath) {
    try {
        addTerminalLog(`📖 Loading file: ${filePath}`, 'info');
        const response = await fetch(`${API_BASE}/files/${encodeURIComponent(filePath)}`);
        if (response.ok) {
            const data = await response.json();
            console.log('File response data:', data);
            
            // Handle different response structures
            // ToolResult structure: {"success": true, "result": {"content": [{"type": "text", "text": "..."}], "is_error": false}}
            // The text field may contain JSON string (for tool results) or direct content
            let content = null;
            
            if (data.result) {
                // Check if it's a ToolResult with content array
                if (data.result.content && Array.isArray(data.result.content)) {
                    // Extract text from content array
                    for (const item of data.result.content) {
                        if (item.type === 'text' && item.text) {
                            // Try to parse as JSON first (in case it's a tool result dict)
                            try {
                                const parsed = JSON.parse(item.text);
                                // If parsed successfully and has content field, use it
                                if (parsed.content) {
                                    content = parsed.content;
                                } else {
                                    // Otherwise use text directly (it's the actual file content)
                                    content = item.text;
                                }
                            } catch (e) {
                                // Not JSON, use text directly (it's the actual file content)
                                content = item.text;
                            }
                            break;
                        }
                    }
                }
                
                // Fallback to direct structure
                if (!content) {
                    if (typeof data.result === 'string') {
                        content = data.result;
                    } else if (data.result.content && typeof data.result.content === 'string') {
                        content = data.result.content;
                    } else if (data.result.result) {
                        if (typeof data.result.result === 'string') {
                            content = data.result.result;
                        } else if (data.result.result.content) {
                            content = data.result.result.content;
                        }
                    }
                }
            } else if (data.content) {
                content = data.content;
            }
            
            if (content) {
                // Create a blob URL and set it as iframe source
                const blob = new Blob([content], { type: 'text/html' });
                const url = URL.createObjectURL(blob);
                previewFrame.src = url;
                addTerminalLog(`✓ Preview updated: ${filePath}`, 'success');
                // Inject styles after load to fit content and disable scrolling
                previewFrame.addEventListener('load', injectPreviewStyles, { once: true });
            } else {
                console.error('No content found in response:', data);
                addTerminalLog(`⚠ No content found in ${filePath}. Response: ${JSON.stringify(data).substring(0, 100)}`, 'warning');
            }
        } else {
            const errorText = await response.text();
            addTerminalLog(`⚠ Failed to load ${filePath}: HTTP ${response.status} - ${errorText}`, 'warning');
        }
    } catch (error) {
        console.error('Error rendering preview:', error);
        addTerminalLog(`✗ Error rendering preview: ${error.message}`, 'error');
    }
}

async function refreshPreview() {
    addTerminalLog('🔄 Refreshing preview...', 'info');
    // Try to find and render the main HTML file
    try {
        const response = await fetch(`${API_BASE}/files`);
        if (response.ok) {
            const data = await response.json();
            console.log('Files list response:', data);
            
            // Handle different response structures
            // ToolResult structure: {"success": true, "result": {"content": [{"type": "text", "text": "..."}], "is_error": false}}
            // The text field contains JSON string with actual data
            let files = [];
            
            if (data.result) {
                // Check if it's a ToolResult with content array
                if (data.result.content && Array.isArray(data.result.content)) {
                    // Extract text from content array and parse JSON
                    for (const item of data.result.content) {
                        if (item.type === 'text' && item.text) {
                            try {
                                const parsed = JSON.parse(item.text);
                                if (parsed.files && Array.isArray(parsed.files)) {
                                    files = parsed.files;
                                    break;
                                }
                            } catch (e) {
                                console.warn('Failed to parse content text:', e);
                            }
                        }
                    }
                }
                
                // Fallback to direct structure
                if (files.length === 0) {
                    if (Array.isArray(data.result)) {
                        files = data.result;
                    } else if (data.result.files) {
                        files = data.result.files;
                    } else if (data.result.result && data.result.result.files) {
                        files = data.result.result.files;
                    } else if (data.result.success && data.result.result && data.result.result.files) {
                        files = data.result.result.files;
                    }
                }
            } else if (data.files) {
                files = data.files;
            }
            
            console.log('Extracted files:', files);
            
            if (files.length === 0) {
                addTerminalLog('⚠ No files found in VFS. Response: ' + JSON.stringify(data).substring(0, 200), 'warning');
                return;
            }
            
            addTerminalLog(`📁 Found ${files.length} file(s): ${files.join(', ')}`, 'info');
            
            // Look for common entry points (including game-related names)
            const entryPoints = ['index.html', 'app.html', 'main.html', 'game.html', 'snake.html'];
            for (const entry of entryPoints) {
                if (files.includes(entry)) {
                    addTerminalLog(`🎯 Found entry point: ${entry}`, 'info');
                    await renderPreview(entry);
                    return;
                }
            }
            
            // If no entry point found, try to render the first HTML file
            const htmlFiles = files.filter(f => f.endsWith('.html'));
            if (htmlFiles.length > 0) {
                addTerminalLog(`📄 Rendering first HTML file: ${htmlFiles[0]}`, 'info');
                await renderPreview(htmlFiles[0]);
            } else {
                addTerminalLog(`⚠ No HTML files found. Available files: ${files.join(', ')}`, 'warning');
            }
        } else {
            const errorText = await response.text();
            addTerminalLog(`⚠ Failed to list files: HTTP ${response.status} - ${errorText}`, 'warning');
        }
    } catch (error) {
        console.error('Error refreshing preview:', error);
        addTerminalLog(`✗ Error refreshing preview: ${error.message}`, 'error');
    }
}

refreshPreviewBtn.addEventListener('click', refreshPreview);

// Disable wheel scrolling on preview frame
previewFrame.addEventListener('wheel', (e) => {
    e.preventDefault();
    e.stopPropagation();
}, { passive: false });

// Also disable scrolling on preview container
const previewContainer = document.querySelector('.preview-container');
if (previewContainer) {
    previewContainer.addEventListener('wheel', (e) => {
        e.preventDefault();
        e.stopPropagation();
    }, { passive: false });
}


// Inject CSS to make iframe content fit and disable scrolling
function injectPreviewStyles() {
    try {
        const iframeDoc = previewFrame.contentDocument || previewFrame.contentWindow.document;
        if (iframeDoc && iframeDoc.head) {
            // Create or update style element
            let styleEl = iframeDoc.getElementById('preview-fit-styles');
            if (!styleEl) {
                styleEl = iframeDoc.createElement('style');
                styleEl.id = 'preview-fit-styles';
                iframeDoc.head.appendChild(styleEl);
            }
            
            // Set styles to fit content and disable scrolling
            styleEl.textContent = `
                html, body {
                    margin: 0;
                    padding: 0;
                    width: 100%;
                    height: 100%;
                    overflow: hidden !important;
                    position: fixed;
                }
                body {
                    display: flex;
                    flex-direction: column;
                }
                * {
                    max-width: 100%;
                    box-sizing: border-box;
                }
            `;
        }
    } catch (e) {
        // Cross-origin or other error, ignore
        console.debug('Cannot inject styles into iframe (cross-origin):', e.message);
    }
}

// Update styles when iframe loads
previewFrame.addEventListener('load', () => {
    setTimeout(injectPreviewStyles, 100);
});

