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