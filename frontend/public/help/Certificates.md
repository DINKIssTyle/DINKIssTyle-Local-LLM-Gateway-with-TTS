<!--lang:en-->
# 🛡️ HTTPS Self-Signed Certificates & Trust Guide

The **DKST LLM Chat Server** provides self-signed HTTPS certificates to ensure all communications are encrypted and secure by default.

## Why do I see a security warning?
Even though the connection is encrypted, browsers show a warning because the certificate is **self-signed**. The browser recognizes the encryption but cannot verify the server's identity because the certificate wasn't issued by a public authority.

## How to trust the Self-Signed Certificate
To eliminate warnings, you must generate a certificate tied to your specific address and install it.

1.  **Preparation:** You need a static IP or a domain name (using a **DDNS** service is highly recommended).
2.  **Generation:** Enter your IP or Domain in the **Certificate Domain / Address** field.

> [!IMPORTANT]
> **Identification:** The generated **filename** (e.g., `your-domain.com.crt`) and the **Certificate Name** displayed in your system settings will match the exact address you enter here.
    
3.  **Activation:** Click **Generate Certificate**. The server applies it immediately.
4.  **Download:** Access your server URL and click **Download Certificate** below the login form.

---

## 📂 CA Certificate Installation by Platform

## 📱 Mobile Devices

### **iOS / iPadOS**
1.  Open the downloaded file and tap **Allow** to download the profile.
2.  Go to **Settings > Profile Downloaded** and tap **Install**.
3.  **Crucial Step:** Go to **Settings > General > About > Certificate Trust Settings**.
4.  Find the certificate named after your **Domain/IP** and toggle it **ON**.

### **Android**
1.  Go to **Settings > Security > Advanced > Encryption & credentials**.
2.  Tap **Install a certificate** > **CA certificate**.
3.  Tap **Install anyway**, select the file (named `your-address.crt`), and save it.

---

## 💻 Desktop Operating Systems

### **macOS**
1.  Double-click the downloaded file to open **Keychain Access**.
2.  Locate the certificate entry (it will be named your **Domain** or **IP**).
3.  Double-click it, expand **Trust**, and set "When using this certificate" to **Always Trust**.

### **Windows**
1.  Double-click the file and click **Install Certificate...**.
2.  Select **Local Machine** > **Place all certificates in the following store**.
3.  Click **Browse** and select **Trusted Root Certification Authorities**.
4.  Confirm the installation. The certificate will appear in your list under your **Domain/IP**.

---

## 🌐 Web Browsers

### **Desktop Chrome**
1.  Go to `chrome://settings/security` > **Manage certificates**.
2.  In the **Authorities** tab, click **Import**.
3.  Select your file and check **Trust this certificate for identifying websites**.

---

> [!TIP]
> Once installed, the browser will recognize your specific **Domain/IP** as a trusted source, and the "Not Secure" warning will be replaced by a **Lock icon**.

<br>
<br>

<!--/lang-->

<!--lang:ko-->

# 🛡️ HTTPS 자가 서명 인증서 및 인증 가이드

**DKST LLM Chat Server**는 모든 통신이 기본적으로 암호화되고 안전하도록 자가 서명된 HTTPS 인증서를 제공합니다.

## 왜 보안 경고가 표시되나요?
연결이 암호화되어 있음에도 불구하고, 브라우저는 인증서가 **자가 서명**되었기 때문에 경고를 표시합니다. 브라우저는 암호화는 인식하지만, 인증서가 공인 기관에서 발행되지 않았기 때문에 서버의 신원을 확인할 수 없습니다.

## 자가 서명 인증서를 신뢰하는 방법
경고를 제거하려면 특정 주소에 연결된 인증서를 생성하고 설치해야 합니다.

1.  **준비:** 고정 IP 또는 도메인 이름이 필요합니다 (**DDNS** 서비스 사용을 강력히 권장합니다).
2.  **생성:** **Certificate Domain / Address** 필드에 IP 또는 도메인을 입력합니다.

> [!IMPORTANT]
> **식별:** 생성된 **파일명**(예: `your-domain.com.crt`)과 시스템 설정에 표시되는 **인증서 이름**은 여기서 입력한 정확한 주소와 일치하게 됩니다.
    
3.  **활성화:** **Generate Certificate**를 클릭합니다. 서버에 즉시 적용됩니다.
4.  **다운로드:** 서버 URL에 접속하여 로그인 폼 아래의 **Download Certificate**를 클릭합니다.

---

## 📂 플랫폼별 CA 인증서 설치 방법

## 📱 모바일 기기

### **iOS / iPadOS**
1.  다운로드한 파일을 열고 **허용**을 눌러 프로파일을 다운로드합니다.
2.  **설정 > 프로파일이 다운로드됨**으로 이동하여 **설치**를 누릅니다.
3.  **중요 단계:** **설정 > 일반 > 정보 > 인증서 신뢰 설정**으로 이동합니다.
4.  사용자의 **도메인/IP** 이름으로 된 인증서를 찾아 **활성화(ON)** 합니다.

### **Android**
1.  **설정 > 보안 > 고급 > 암호화 및 자격 증명**으로 이동합니다.
2.  **인증서 설치** > **CA 인증서**를 누릅니다.
3.  **그대로 설치**를 누르고, 파일(`your-address.crt`)을 선택하여 저장합니다.

---

## 💻 데스크톱 운영 체제

### **macOS**
1.  다운로드한 파일을 더블 클릭하여 **키체인 접근**을 엽니다.
2.  인증서 항목을 찾습니다 (사용자의 **도메인** 또는 **IP** 이름으로 되어 있습니다).
3.  항목을 더블 클릭하고, **신뢰** 섹션을 확장한 후 "이 인증서 사용 시"를 **항상 신뢰**로 설정합니다.

### **Windows**
1.  파일을 더블 클릭하고 **인증서 설치...**를 클릭합니다.
2.  **로컬 컴퓨터** > **모든 인증서를 다음 저장소에 저장**을 선택합니다.
3.  **찾아보기**를 클릭하고 **신뢰할 수 있는 루트 인증 기관**을 선택합니다.
4.  설치를 확인합니다. 인증서는 사용자의 **도메인/IP** 이름으로 목록에 나타납니다.

---

## 🌐 웹 브라우저

### **Desktop Chrome**
1.  `chrome://settings/security` > **기기 인증서 관리**로 이동합니다.
2.  **신뢰할 수 있는 인증 기관** 탭에서 **가져오기**를 클릭합니다.
3.  파일을 선택하고 **이 인증서를 웹사이트 식별에 사용하도록 신뢰**를 체크합니다.

---

> [!TIP]
> 설치가 완료되면 브라우저는 사용자의 특정 **도메인/IP**를 신뢰할 수 있는 소스로 인식하며, "주의 요함" 경고 대신 **자물쇠 아이콘**이 표시됩니다.

<!--/lang-->