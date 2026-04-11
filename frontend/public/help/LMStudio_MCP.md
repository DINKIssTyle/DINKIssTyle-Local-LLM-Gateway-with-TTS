The **DKST LLM Chat Server** provides various MCP (Model Context Protocol) tools. 
To integrate **LM Studio** with these tools, you need to configure the settings **within the LM Studio application** as follows:

---

# 🛠️ LM Studio Integration Procedure

## 1. Access Developer Settings
Navigate to the **Developer Tab** in LM Studio by pressing:
* `Ctrl + 2` (Windows/Linux), 
* `Cmd + 2` (macOS)

## 2. Configure Server Settings
Click on **Server Settings** and enable the following toggle switches:
* **Require Authentication**
* **Allow calling servers from mcp.json**

## 3. Update Configuration File
Click on **mcp.json** to open the editor, then:
* Copy and paste the required JSON configuration into the **Edit mcp.json** field.
* **Save** the file to apply the changes.

