package models

// CertificateSigningRequest is a certificate signing request
type CertificateSigningRequest struct {

	// Requested 'duration' (i.e. lifetime) of the Certificate. Note that the
	// issuer may choose to ignore the requested duration, just like any other
	// requested attribute.
	// +optional
	Duration *Duration `json:"duration,omitempty" swaggertype:"string"`

	// The PEM-encoded X.509 certificate signing request to be submitted to the
	// issuer for signing.
	//
	// If the CSR has a BasicConstraints extension, its isCA attribute must
	// match the `isCA` value of this CertificateRequest.
	// If the CSR has a KeyUsage extension, its key usages must match the
	// key usages in the `usages` field of this CertificateRequest.
	// If the CSR has a ExtKeyUsage extension, its extended key usages
	// must match the extended key usages in the `usages` field of this
	// CertificateRequest.
	Request string `json:"request" example:"-----BEGIN CERTIFICATE REQUEST-----(...)-----END CERTIFICATE REQUEST-----"`

	// Requested basic constraints isCA value. Note that the issuer may choose
	// to ignore the requested isCA value, just like any other requested attribute.
	//
	// NOTE: If the CSR in the `Request` field has a BasicConstraints extension,
	// it must have the same isCA value as specified here.
	//
	// If true, this will automatically add the `cert sign` usage to the list
	// of requested `usages`.
	// +optional
	IsCA bool `json:"is_ca,omitempty"`

	// Requested key usages and extended key usages.
	//
	// NOTE: If the CSR in the `Request` field has uses the KeyUsage or
	// ExtKeyUsage extension, these extensions must have the same values
	// as specified here without any additional values.
	//
	// If unset, defaults to `digital signature` and `key encipherment`.
	// +optional
	Usages []KeyUsage `json:"usages,omitempty"`
}

// CertificateSigningResponse is a certificate signing response
type CertificateSigningResponse struct {

	// The PEM encoded X.509 certificate resulting from the certificate
	// signing request.
	// If not set, the CertificateRequest has either not been completed or has
	// failed. More information on failure can be found by checking the
	// `conditions` field.
	// +optional
	Certificate string `json:"certificate,omitempty" example:"-----BEGIN CERTIFICATE-----(...)-----END CERTIFICATE-----"`

	// The PEM encoded X.509 certificate of the signer, also known as the CA
	// (Certificate Authority).
	// This is set on a best-effort basis by different issuers.
	// If not set, the CA is assumed to be unknown/not available.
	// +optional
	CA string `json:"ca,omitempty" example:"-----BEGIN CERTIFICATE-----(...)-----END CERTIFICATE-----"`
}

type KeyUsage string

const (
	UsageSigning           KeyUsage = "signing"
	UsageDigitalSignature  KeyUsage = "digital signature"
	UsageContentCommitment KeyUsage = "content commitment"
	UsageKeyEncipherment   KeyUsage = "key encipherment"
	UsageKeyAgreement      KeyUsage = "key agreement"
	UsageDataEncipherment  KeyUsage = "data encipherment"
	UsageCertSign          KeyUsage = "cert sign"
	UsageCRLSign           KeyUsage = "crl sign"
	UsageEncipherOnly      KeyUsage = "encipher only"
	UsageDecipherOnly      KeyUsage = "decipher only"
	UsageAny               KeyUsage = "any"
	UsageServerAuth        KeyUsage = "server auth"
	UsageClientAuth        KeyUsage = "client auth"
	UsageCodeSigning       KeyUsage = "code signing"
	UsageEmailProtection   KeyUsage = "email protection"
	UsageSMIME             KeyUsage = "s/mime"
	UsageIPsecEndSystem    KeyUsage = "ipsec end system"
	UsageIPsecTunnel       KeyUsage = "ipsec tunnel"
	UsageIPsecUser         KeyUsage = "ipsec user"
	UsageTimestamping      KeyUsage = "timestamping"
	UsageOCSPSigning       KeyUsage = "ocsp signing"
	UsageMicrosoftSGC      KeyUsage = "microsoft sgc"
	UsageNetscapeSGC       KeyUsage = "netscape sgc"
)
