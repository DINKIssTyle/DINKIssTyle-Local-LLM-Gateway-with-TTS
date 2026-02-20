import sys
import os
import ipaddress
from datetime import datetime, timedelta

from PyQt5.QtWidgets import (
    QApplication, QWidget, QLabel, QLineEdit,
    QPushButton, QFileDialog, QVBoxLayout,
    QMessageBox
)

from cryptography import x509
from cryptography.x509.oid import NameOID, ExtendedKeyUsageOID
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.backends import default_backend


class PrivateCAGenerator(QWidget):

    def __init__(self):
        super().__init__()
        self.setWindowTitle("Private Root CA Generator")
        self.setMinimumWidth(420)
        self.init_ui()

    def init_ui(self):
        layout = QVBoxLayout()

        label = QLabel("Domain Name (Production):")
        layout.addWidget(label)

        self.domain_input = QLineEdit()
        self.domain_input.setPlaceholderText("example.com")
        layout.addWidget(self.domain_input)

        self.generate_btn = QPushButton("Generate Certificates")
        self.generate_btn.clicked.connect(self.generate_certificates)
        layout.addWidget(self.generate_btn)

        self.setLayout(layout)

    def generate_certificates(self):
        domain = self.domain_input.text().strip()

        if not domain:
            QMessageBox.warning(self, "Error", "Please enter a valid domain name.")
            return

        save_dir = QFileDialog.getExistingDirectory(
            self, "Select Directory to Save Certificates"
        )

        if not save_dir:
            return

        try:
            # -------------------------------------------------
            # 1️⃣ Generate Root CA Key (4096 RSA)
            # -------------------------------------------------
            ca_key = rsa.generate_private_key(
                public_exponent=65537,
                key_size=4096,
                backend=default_backend()
            )

            subject = issuer = x509.Name([
                x509.NameAttribute(NameOID.COUNTRY_NAME, "US"),
                x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Private Certificate Authority"),
                x509.NameAttribute(NameOID.COMMON_NAME, "Private Root CA"),
            ])

            ca_builder = (
                x509.CertificateBuilder()
                .subject_name(subject)
                .issuer_name(issuer)
                .public_key(ca_key.public_key())
                .serial_number(x509.random_serial_number())
                .not_valid_before(datetime.utcnow() - timedelta(days=1))
                .not_valid_after(datetime.utcnow() + timedelta(days=3650))  # 10 years
                .add_extension(
                    x509.BasicConstraints(ca=True, path_length=None),
                    critical=True
                )
                .add_extension(
                    x509.KeyUsage(
                        digital_signature=False,
                        content_commitment=False,
                        key_encipherment=False,
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
                    x509.SubjectKeyIdentifier.from_public_key(ca_key.public_key()),
                    critical=False
                )
                .add_extension(
                    x509.AuthorityKeyIdentifier.from_issuer_public_key(ca_key.public_key()),
                    critical=False
                )
            )

            ca_cert = ca_builder.sign(
                private_key=ca_key,
                algorithm=hashes.SHA256(),
                backend=default_backend()
            )

            # -------------------------------------------------
            # 2️⃣ Generate Server Key (RSA 2048)
            # -------------------------------------------------
            server_key = rsa.generate_private_key(
                public_exponent=65537,
                key_size=2048,
                backend=default_backend()
            )

            # SAN (Production: only domain)
            alt_names = []

            try:
                # If domain is IP
                alt_names.append(
                    x509.IPAddress(ipaddress.ip_address(domain))
                )
            except ValueError:
                alt_names.append(x509.DNSName(domain))

            san = x509.SubjectAlternativeName(alt_names)

            server_builder = (
                x509.CertificateBuilder()
                .subject_name(
                    x509.Name([
                        x509.NameAttribute(NameOID.COMMON_NAME, domain)
                    ])
                )
                .issuer_name(ca_cert.subject)
                .public_key(server_key.public_key())
                .serial_number(x509.random_serial_number())
                .not_valid_before(datetime.utcnow() - timedelta(days=1))
                .not_valid_after(datetime.utcnow() + timedelta(days=398))  # Browser compliant
                .add_extension(san, critical=False)
                .add_extension(
                    x509.BasicConstraints(ca=False, path_length=None),
                    critical=True
                )
                .add_extension(
                    x509.KeyUsage(
                        digital_signature=True,
                        content_commitment=False,
                        key_encipherment=True,
                        data_encipherment=False,
                        key_agreement=False,
                        key_cert_sign=False,
                        crl_sign=False,
                        encipher_only=False,
                        decipher_only=False
                    ),
                    critical=True
                )
                .add_extension(
                    x509.ExtendedKeyUsage(
                        [ExtendedKeyUsageOID.SERVER_AUTH]
                    ),
                    critical=False
                )
                .add_extension(
                    x509.SubjectKeyIdentifier.from_public_key(server_key.public_key()),
                    critical=False
                )
                .add_extension(
                    x509.AuthorityKeyIdentifier.from_issuer_public_key(ca_key.public_key()),
                    critical=False
                )
            )

            server_cert = server_builder.sign(
                private_key=ca_key,
                algorithm=hashes.SHA256(),
                backend=default_backend()
            )

            # -------------------------------------------------
            # 3️⃣ Save Files
            # -------------------------------------------------
            with open(os.path.join(save_dir, "rootCA.key"), "wb") as f:
                f.write(
                    ca_key.private_bytes(
                        encoding=serialization.Encoding.PEM,
                        format=serialization.PrivateFormat.TraditionalOpenSSL,
                        encryption_algorithm=serialization.NoEncryption()
                    )
                )

            with open(os.path.join(save_dir, "rootCA.pem"), "wb") as f:
                f.write(ca_cert.public_bytes(serialization.Encoding.PEM))

            with open(os.path.join(save_dir, f"{domain}.key"), "wb") as f:
                f.write(
                    server_key.private_bytes(
                        encoding=serialization.Encoding.PEM,
                        format=serialization.PrivateFormat.TraditionalOpenSSL,
                        encryption_algorithm=serialization.NoEncryption()
                    )
                )

            with open(os.path.join(save_dir, f"{domain}.crt"), "wb") as f:
                f.write(server_cert.public_bytes(serialization.Encoding.PEM))

            QMessageBox.information(
                self,
                "Success",
                "Certificates generated successfully.\n\n"
                "IMPORTANT:\n"
                "Install rootCA.pem into your OS trust store "
                "to avoid browser warnings."
            )

        except Exception as e:
            QMessageBox.critical(self, "Error", str(e))


if __name__ == "__main__":
    app = QApplication(sys.argv)
    window = PrivateCAGenerator()
    window.show()
    sys.exit(app.exec_())