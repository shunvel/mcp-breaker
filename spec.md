# Specification: Semantic MCP Circuit Breaker (mcp-breaker)

## 1. Product Overview & Problem Statement
Autonomous AI agents utilizing Model Context Protocol (MCP) frequently encounter **Semantic Stagnation Loops**. Unlike standard code loops that crash or throw exceptions, an AI logic loop returns a `200 OK` status network payload on every turn. The agent runs successfully from an engineering perspective, but its reasoning is locked in a recursive pattern (e.g., executing the same terminal compile command, rewriting files with synonymous words, or calling identical tool parameters repeatedly).

### The Technical Pain Points
1. **Ineffective Standard Guardrails:** Standard numeric limits (`max_turns = 20`) act too late. By the time they trigger, significant API credits are drained, and the LLM's context window is flooded with repetitive logs.
2. **Context Degradation:** As the loop propagates, short-term memory fills with garbage responses, preventing the agent from autonomously breaking free.
3. **Hard Failures vs. Graceful Re-routing:** Existing tools hard-abort executions when limits are reached, resulting in degraded user experiences rather than error recovery or system interventions.

**mcp-breaker** is an open-source, lightweight JSON-RPC proxy and SDK middleware that monitors tool invocation paths and state transformations. It flags semantic loops within **2 to 3 iterations** and dynamically modifies execution tracks without terminating active agent chat sessions.

---

## 2. System Architecture & Matrix

The system runs as an interception layer between the AI Client (e.g., Cursor, Claude Desktop, Cline) and target backend MCP servers.

```
+-----------+    JSON-RPC (tools/call)   +-----------------+    Proxied Call    +------------+
| AI Client | ─────────────────────────> |   mcp-breaker   | ─────────────────> | MCP Server |
|  (Cursor) | <───────────────────────── | (Proxy Layer)   | <───────────────── |  (Target)  |
+-----------+    System Intervention     +-----------------+    Tool Response   +------------+
                          ▲
                          │ Calculates Distance
                          ▼
                 +-----------------+
                 |  Local Tracker  |
                 | (Vector & Echo) |
                 +-----------------+
```

### Core Execution Matrix

| Interception Module | Input Checked | Detection Algorithm | Breaker Action |
| :--- | :--- | :--- | :--- |
| **Tool Echo Detector** | JSON-RPC string arguments | Strict string matching & exact hash equality over time windows ($T-3$). | Rewrites payload to inject structural warnings or pauses execution paths. |
| **Semantic State Tracker** | LLM thought text / tool log outputs | Text distance using lightweight semantic models (Cosine Similarity threshold $\ge 0.88$). | Intercepts return payloads and appends clear System Intervention Prompts. |
| **Sliding Window Ledger** | Sequence of the last $N$ turns | Graph loop state mapping ($A 	o B 	o A 	o B$). | Hard-trips to user confirmation portal via CLI / Local Interface. |

---

## 3. Detailed Feature Specifications

### 3.1. Tool Parameter Echo Monitoring (Zero-Overhead Static Gate)
* **Behavior:** Intercepts every incoming `tools/call` JSON-RPC request.
* **Mechanism:** Maintains an in-memory hash ring of the last 5 executed tools grouped by target method names. If `method: "execute_command"` passes arguments `{"cmd": "npm run test"}` continuously 3 times, the breaker immediately activates.
* **Action:** Bypasses target server invocation entirely and immediately returns a mock context layer back to the client: `"Error: Command [npm run test] generated identical failures across consecutive loops. Do not retry without modifying parameters."`

### 3.2. Semantic Trajectory Analysis (Vector Distance Tracking)
* **Behavior:** Evaluates qualitative stagnation when tool payloads vary slightly in text formatting but remain conceptually identical.
* **Mechanism:** Converts tool returns and agent responses into vectors via a highly efficient local tokenizer/embedding engine.
* **Algorithmic Trigger:** 
  $$\text{Cosine Similarity} = \frac{A \cdot B}{\|A\| \|B\|}$$
  If the similarity score between Turn $N$ and Turn $N-2$ is $\ge 0.88$, the system registers a trajectory violation flag.

### 3.3. Graceful Degradation & System Prompt Intervention
* **Behavior:** Prevents hard system crashes. Rather than failing the workflow, it realigns agent strategies.
* **Mechanism:** If a semantic loop or echo is flagged, the proxy intercepts the message path and builds an augmented system context turn:
  ```json
  {
    "status": "success_with_intervention",
    "system_override_notice": "[CRITICAL REASONING ALERT] You have entered a semantic loop. You are continuously requesting the same files without creating distinct outcomes. Step back, abandon your current path, check alternate files, or request explicit clarification from the user."
  }
  ```

---

## 4. Preferred Tech Stack & Technical Justification

* **Core Implementation Language:** **Go (Golang)**
  * *Justification:* Compiles to a single, zero-dependency native binary, ensuring lightning-fast execution times. It can process high-throughput JSON-RPC streams over `stdio` or WebSockets without introducing human-perceptible latency to IDE setups.
* **Local Embedding Engine:** **ONNX Runtime (with a minimized All-MiniLM-L6-v2 model package)**
  * *Justification:* Requires complete privacy and offline utility. Running a quantized, local 25MB ONNX vector embedding engine inside Go yields feature extraction times under **3ms per text block**, eliminating the need for expensive API network calls just to verify semantic similarity.
* **Storage Layer:** **In-Memory SQLite (WAL Mode enabled) or Custom Ring Buffers**
  * *Justification:* State history tracking is ephemeral and bound strictly to the active development session. An in-memory ledger keeps performance footprints minimal while allowing developers to write flexible SQL queries to analyze loop histories.

---

## 5. Distribution Methodology

To achieve developers' trust and organic, open-source adoption, deployment is designed for instant onboarding:

1. **Standalone Homebrew / Shell Installer:**
   ```bash
   brew install mcp-breaker
   ```
2. **Global Config Wrapper Mode:**
   `mcp-breaker` exposes a CLI initialization command (`mcp-breaker init`) that automatically parses your environment's native MCP config file (such as `claude_desktop_config.json` or Cursor's local configuration). It injects itself into the path as a middleware router:
   ```json
   // BEFORE:
   "postgres-server": { "command": "node", "args": ["postgres_mcp.js"] }
   
   // AFTER AUTO-WRAP:
   "postgres-server": { "command": "mcp-breaker", "args": ["--wrap", "node postgres_mcp.js"] }
   ```
3. **Local Developer TUI / Dashboard:**
   Running `mcp-breaker dashboard` fires up a clean, Terminal User Interface (TUI) mapping real-time streams, calculated cosine distances, and total token dollars protected from looping failures.

---

## 6. Verification Test Cases for Cursor Code Generation

Ensure the generated code successfully executes against these verification profiles:
* **Test Case A (Echo Check):** Mock an LLM that calls `write_file` with identical content strings 3 times consecutively. The test passes if the proxy blocks the 3rd network request and yields an intentional intervention schema.
* **Test Case B (Semantic Stagnation):** Pass three logs that vary in formatting but describe the same logical error (`"Port 8080 bound"`, `"Error: listen EADDRINUSE: address already in use 8080"`, `"Cannot bind network to 8080"`). The test passes if the Cosine Vector module identifies the semantic loop and triggers the system override sequence.