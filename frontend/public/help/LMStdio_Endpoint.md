<!--lang:en-->
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
<!--/lang-->

<!--lang:ko-->
# 🛠️ LLM 연동 방법

## 1. LM Studio와 연결하기 (권장)

**LM Studio** 앱 안에서 아래 순서대로 진행하세요.

* **Developer Tab 열기**
    * Windows/Linux: `Ctrl + 2`
    * macOS: `Cmd + 2`

* **서버 시작**
    * **Status: Stopped** 옆 토글을 켜서 로컬 서버를 시작합니다.
    * **Reachable at** 주소 옆의 **Copy** 버튼을 눌러 주소를 복사합니다.
    * 다시 **DKST LLM Chat Server**로 돌아와 **LLM Endpoint** 칸에 붙여 넣습니다.

---

**다시 LM Studio에서:**

* **Server Settings 설정**
    * 필요하면 **Server Port**를 바꿀 수 있습니다.
    * **Require Authentication**을 **ON**으로 설정합니다.

* **API 토큰 생성**
    1. **Manage Tokens**를 클릭합니다.
    2. **+ Create new token**을 클릭합니다.
    3. **Allow calling servers from mcp.json**을 **Allow**로 설정한 뒤 **Create token**을 누릅니다.
    4. 새로 만든 토큰 옆 **Copy** 버튼으로 복사합니다.

---

**다시 DKST LLM Chat Server에서:**

* **설정 마무리**
    * 복사한 토큰을 **API Key** 칸에 붙여 넣습니다.

---

## 2. OpenAI 호환 런타임과 연결하기

* **Ollama**
    * Ollama 같은 OpenAI 호환 API도 로컬 호스트 주소와 해당 모델 이름을 입력해 연결할 수 있습니다.
<!--/lang-->
