import sys
import os
import ipaddress
from datetime import datetime, timedelta

from PyQt5.QtWidgets import (
    QApplication, QWidget, QLabel, QLineEdit,
    QPushButton, QFileDialog, QVBoxLayout,
    QMessageBox, QHBoxLayout
)

from cryptography import x509
from cryptography.x509.oid import NameOID, ExtendedKeyUsageOID
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa


# === Must match server.go behavior ===
ORG_NAME = "DINKI'ssTyle Local LLM Gateway"
VALID_DAYS = 365  # server.go: 1 year


def generate_servergo_style_cert(cert_domain: str, out_dir: str) -> dict:
    """
    Replicates server.go ensureSelfSignedCert() certificate template:
    - RSA 2048
    - Subject: O=ORG_NAME, CN=cert_domain
    - Self-signed (issuer=subject)
    - NotBefore=now, NotAfter=now+365d
    - KeyUsage: KeyEncipherment | DigitalSignature | CertSign | CRLSign
    - ExtKeyUsage: ServerAuth
    - BasicConstraintsValid=true, IsCA=true
    - SAN:
        IP: 127.0.0.1
        IP: ::1
        DNS: localhost
        DNS: cert_domain (ONLY if cert_domain != localhost && cert_domain != 127.0.0.1)
      (server.go appends cert_domain as DNSName, even if it's an IP string except 127.0.0.1)
    - Writes:
        <domain>.crt      (PEM certificate)
        <domain>.key      (PEM PKCS#8 private key, "PRIVATE KEY")
        <domain>.der.crt  (DER certificate, for Android-friendly install)
    """
    cert_domain = (cert_domain or "").strip()
    if not cert_domain:
        raise ValueError("Domain is empty.")
    if not os.path.isdir(out_dir):
        raise ValueError("Output directory is invalid.")

    # --- Key (RSA 2048) ---
    priv = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    not_before = datetime.utcnow()
    not_after = not_before + timedelta(days=VALID_DAYS)

    subject = x509.Name([
        x509.NameAttribute(NameOID.ORGANIZATION_NAME, ORG_NAME),
        x509.NameAttribute(NameOID.COMMON_NAME, cert_domain),
    ])

    # --- Base template (server.go style) ---
    builder = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(subject)  # self-signed
        .public_key(priv.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(not_before)
        .not_valid_after(not_after)
        .add_extension(
            x509.BasicConstraints(ca=True, path_length=None),
            critical=True
        )
        .add_extension(
            x509.KeyUsage(
                digital_signature=True,
                content_commitment=False,
                key_encipherment=True,
                data_encipherment=False,
                key_agreement=False,
                key_cert_sign=True,
                crl_sign=True,
                encipher_only=False,
                decipher_only=False
            ),
            critical=True
        )
        .add_extension(
            x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]),
            critical=False
        )
    )

    # --- SANs (match server.go) ---
    san_list = [
        x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
        x509.IPAddress(ipaddress.ip_address("::1")),
        x509.DNSName("localhost"),
    ]

    if cert_domain != "localhost" and cert_domain != "127.0.0.1":
        # server.go adds certDomain to DNSNames (not IPAddresses)
        san_list.append(x509.DNSName(cert_domain))

    builder = builder.add_extension(
        x509.SubjectAlternativeName(san_list),
        critical=False
    )

    # (Optional but safe) SKI/AKI improves compatibility; doesn't change the "style" meaningfully
    builder = builder.add_extension(
        x509.SubjectKeyIdentifier.from_public_key(priv.public_key()),
        critical=False
    )
    builder = builder.add_extension(
        x509.AuthorityKeyIdentifier.from_issuer_public_key(priv.public_key()),
        critical=False
    )

    cert = builder.sign(private_key=priv, algorithm=hashes.SHA256())

    # --- Filenames ---
    safe_name = cert_domain.replace("/", "_").replace("\\", "_").replace(":", "_").strip()
    cert_pem_path = os.path.join(out_dir, f"{safe_name}.crt")
    key_pem_path = os.path.join(out_dir, f"{safe_name}.key")
    cert_der_path = os.path.join(out_dir, f"{safe_name}.der.crt")

    # --- Write PEM cert ---
    with open(cert_pem_path, "wb") as f:
        f.write(cert.public_bytes(serialization.Encoding.PEM))

    # --- Write PEM key (PKCS#8, "PRIVATE KEY") ---
    with open(key_pem_path, "wb") as f:
        f.write(
            priv.private_bytes(
                encoding=serialization.Encoding.PEM,
                format=serialization.PrivateFormat.PKCS8,  # matches Go x509.MarshalPKCS8PrivateKey
                encryption_algorithm=serialization.NoEncryption(),
            )
        )

    # --- Write DER cert (Android-friendly install) ---
    with open(cert_der_path, "wb") as f:
        f.write(cert.public_bytes(serialization.Encoding.DER))

    return {
        "domain": cert_domain,
        "cert_pem": cert_pem_path,
        "key_pem": key_pem_path,
        "cert_der": cert_der_path,
        "valid_days": VALID_DAYS,
        "org": ORG_NAME,
    }


class MakeCertGUI(QWidget):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("DKST Certificate Generator (server.go style)")
        self.setMinimumWidth(560)
        self._init_ui()

    def _init_ui(self):
        layout = QVBoxLayout()

        title = QLabel("Generate a self-signed CA+Server certificate (server.go style)")
        title.setWordWrap(True)
        layout.addWidget(title)

        row = QHBoxLayout()
        row.addWidget(QLabel("Domain:"))
        self.domain_input = QLineEdit()
        self.domain_input.setPlaceholderText("example.com")
        row.addWidget(self.domain_input)
        layout.addLayout(row)

        self.btn_generate = QPushButton("Generate…")
        self.btn_generate.clicked.connect(self.on_generate)
        layout.addWidget(self.btn_generate)

        notes = QLabel(
            "Output files:\n"
            "• <domain>.crt (PEM certificate)\n"
            "• <domain>.key (PEM PKCS#8 private key)\n"
            "• <domain>.der.crt (DER certificate, recommended for Android install)\n\n"
            "SAN includes: 127.0.0.1, ::1, localhost, and your domain (if not localhost/127.0.0.1)."
        )
        notes.setWordWrap(True)
        layout.addWidget(notes)

        self.setLayout(layout)

    def on_generate(self):
        domain = self.domain_input.text().strip()
        if not domain:
            QMessageBox.warning(self, "Error", "Please enter a domain.")
            return

        out_dir = QFileDialog.getExistingDirectory(self, "Select Output Folder")
        if not out_dir:
            return

        try:
            res = generate_servergo_style_cert(domain, out_dir)

            QMessageBox.information(
                self,
                "Success",
                "Certificates generated successfully.\n\n"
                f"PEM cert : {res['cert_pem']}\n"
                f"PEM key  : {res['key_pem']}\n"
                f"DER cert : {res['cert_der']}\n\n"
                "Android install tip:\n"
                "• Use the *CA certificate* install menu.\n"
                "• If Android asks for a private key, you likely chose 'VPN and apps' or 'User certificate'.\n"
                "• Try installing the *.der.crt from:\n"
                "  Settings → Security → Encryption & credentials → Install a certificate → CA certificate"
            )
        except Exception as e:
            QMessageBox.critical(self, "Error", str(e))


def main():
    app = QApplication(sys.argv)
    w = MakeCertGUI()
    w.show()
    sys.exit(app.exec_())


if __name__ == "__main__":
    main()