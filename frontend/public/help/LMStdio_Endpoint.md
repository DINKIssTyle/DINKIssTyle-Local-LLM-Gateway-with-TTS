# 🛠️ How to Integrate with LLMs

## 1. Connecting with LM Studio (Recommended)

To establish a connection, follow these steps within the **LM Studio** application:

* **Access Developer Tab** Navigate to the **Developer Tab** by pressing:
    * `Ctrl + 2` (Windows/Linux)
    * `Cmd + 2` (macOS)

* **Start the Server** Toggle the switch next to **Status: Stopped** to start the local server.
    Click the **Copy** button next to the **Reachable at** address, then return to the **DKST LLM Chat Server** and paste it into the **LLM Endpoint** field.

---

**Back in LM Studio:**

* **Configure Server Settings** Click on **Server Settings** to adjust the following:
    * **Server Port:** You may change the port number here if necessary.
    * **Authentication:** Toggle **Require Authentication** to **ON**.

* **Generate API Token** 1. Click the **Manage Tokens** button.
    2. Click **+ Create new token**.
    3. Set **Allow calling servers from mcp.json** to **Allow**, then click **Create token**.
    4. Click the **Copy** button next to the newly generated token.

---

**Back in DKST LLM Chat Server:**

* **Finalize Setup** Paste the copied token into the **API Key** field.

---

## 2. Connecting with OpenAI-Compatible Runtimes

* **Ollama**
    * You can also connect via other OpenAI-compatible APIs, such as Ollama, by providing the local host address and the corresponding model name.