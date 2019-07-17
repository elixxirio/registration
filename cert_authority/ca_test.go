package cert_authority

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/registration/testkeys"
	"io/ioutil"
	"testing"
)

//Maybe simplify sign, move these to tests? Thoughts?
func loadCertificate(file string) *x509.Certificate {
	pemEncodedBlock, err := ioutil.ReadFile(file)
	if err != nil {
		jww.ERROR.Printf(err.Error())
	}

	certDecoded, _ := pem.Decode(pemEncodedBlock)
	if certDecoded == nil {
		jww.ERROR.Printf("Decoding PEM Failed For %v", file)
	}

	cert, err := x509.ParseCertificate(certDecoded.Bytes)

	if err != nil {
		jww.ERROR.Printf(err.Error())
	}

	return cert

}

func loadCertificateRequest(file string) *x509.CertificateRequest {
	pemEncodedBlock, err := ioutil.ReadFile(file)
	if err != nil {
		jww.ERROR.Printf(err.Error())
	}

	certDecoded, _ := pem.Decode(pemEncodedBlock)
	if certDecoded == nil {
		jww.ERROR.Printf("Decoding PEM Failed For %v", file)
	}
	fmt.Println(certDecoded.Headers)
	cert, err := x509.ParseCertificateRequest(certDecoded.Bytes)
	if err != nil {
		jww.ERROR.Printf(err.Error())
	}

	return cert
}

//hacked by making it return an interface
//TODO make sure it returns a ecdsa.private key (ie that it is pkcs8)\
//loadPrivKey takes the file given and returns a private key of type ecdsa or rs
func loadPrivKey(file string) interface{} {
	pemEncodedBlock, err := ioutil.ReadFile(file)
	if err != nil {
		jww.ERROR.Printf(err.Error())
	}
	certDecoded, _ := pem.Decode(pemEncodedBlock)
	if certDecoded == nil {
		jww.ERROR.Printf("Decoding PEM Failed For %v", file)
	}

	//Openssl creates pkcs8 by default now...
	privateKey, err := x509.ParsePKCS8PrivateKey(certDecoded.Bytes)

	if err != nil {
		jww.ERROR.Printf(err.Error())
	}
	fmt.Println("above an error")
	return privateKey
}

func getKnownSignature() *pem.Block {
	fileIo := []byte(`-----BEGIN CERTIFICATE-----
MIIFVjCCAz6gAwIBAgIBAjANBgkqhkiG9w0BAQsFADCBkjELMAkGA1UEBhMCVVMx
CzAJBgNVBAgMAkNBMRIwEAYDVQQHDAlDbGFyZW1vbnQxEDAOBgNVBAoMB0VsaXh4
aXIxFDASBgNVBAsMC0RldmVsb3BtZW50MRkwFwYDVQQDDBBnYXRld2F5LmNtaXgu
cmlwMR8wHQYJKoZIhvcNAQkBFhBhZG1pbkBlbGl4eGlyLmlvMB4XDTE5MDcxNjIz
MTkzMloXDTE5MDcxNzIzMTkzMlowADCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCC
AgoCggIBAMXOJ4lDDe2USdfy8uPTiIXbQ/e4k5nXwRuktTAsbqzjiFfqs8Z8WczJ
NTy9vHYlFJhxCTldPT9GDk5dHh8ZalYBnjoMtetW5jTcKH1KHY61LgWp3tFAMQRP
nnvHStpp+glNLHKDQZz+63UwdajbjlLWVE65yclqNj+P2h3ItIkpMIoVVgkqP69W
A5SbEXWm8OEYUx5UuYIsQUmxW+ftkSq6Enzz9uv+Z1bcGjUmnAhQ2rR8/hCV+41c
hGzIIZ6DvQClzvINK+dlaNObx55OzzCXy3n9RBtSmUEQTtTeKu+H1QeMKJh+s0/9
AnNU5QT8yqzxV03oItntS14WyjXfc0aWBanMkgD/D7MzbOaNoi34BTMNnusZ9PCt
Jd05ohYQptHwgcMqpVeWvG2dF4wCPb+C9apvKgGYism7LVJFghhtpCVGmcWf1QZN
WorSX/teHG+CFwEcLLkuUK+EvFQDt0IPqp+cGf/hc/YQdj6vMWB85ZAwodoviCYH
2zllkr56LWabv14IIDwhVxY3zIyEF0GtNe/R88zhB0aMPsGgwHU5qYVgDzUmk35+
O2Cn6y8w3rIRsW5tloNFhAelIEexK8JE5p0Kzv3scT2e4+GcKY4cqNIC6py0vkun
9P9VSKIHavRVgIJ7GoMX8BwfppoGfI/kqWbl5im+9jjbz3sMXzTdAgMBAAGjSDBG
MA4GA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDAjAfBgNVHSMEGDAW
gBTBZRWWVIYwzrQ8wcgvcW8s38RUQzANBgkqhkiG9w0BAQsFAAOCAgEAKsqFlNQK
XPXaxGlLvmRCzz2LQidkektj6jV6AxNOhhhkY+0lHSf7JPN2JE9IqdH4YSuqSx/z
YK2t9NDv8zgUvkyL9m4IDPDja+8VFGw8wVUC4Oa6LZTGfzL7u6NZtqg2xNX1PXMs
t6y8x0Idnj6n16QFS8w+vQDxAmn4UOtDd4MOt7TUvrHsfNbF4+6QRW2EttjvLOHP
/y+JFi4LKYEvSq+FSImuzbNjc2MbclGK/QUR7LL99xa90JjEzKshIvbWs0hglufl
I05s7sxsoCvMXwDftj6onCP780+XERAjA9pXZAkaqsLxJ+eHiwntiYd+nS6edCb8
+CihW2kPjJ3YgdHa82jCkcT/qMZRKsel4csK67CqTtPgX3MnDV/gLvh2VclrZjab
rjsuxzGkrKI3RBouJShVxEVfS+4wxV7fsG73lLV0lehCp8ZVIlSkw9Y6wa5OciD2
yzj+M4m6C+bsxUV9Foi++ow+L8tJ35sP1v/OV5+GnI0VZPsvLmkk2eqCwgHECCqO
CnGgEV7kMbIJm53Ooy/nDxpXawRSlRjbAVnEmLAKy7iSYBOucx+BQ/3TnTQ9S7Ii
XObTGJ8pmDRq9vobLxvxZ6v5wle8nEef5HZW2ddcBQ/2cQdJNIgi7DJi86qj9gc1
8ScD4Dr1Gt4wnORAq0jHkl45CNICTCoplY0=
-----END CERTIFICATE-----`)
	ret, _ := pem.Decode(fileIo)
	return ret
}

//The previously signed certificate in testkeys was generated using the following commands
//openssl x509 -req -days 360 -in <CSR> -CA <CA-CERT> -CAkey <CA-KEY> -CAcreateserial -out alreadySigned.crt -sha256
//The inputs (CA cert/key & CSR) were generated unrelated to the ones in testkeys (ie the following was run twice)
/*		CA TLS keypair generation		*/
//openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 \
//-nodes -out <CA-CERT> -keyout <CA-KEY> -subj <CA-SUBJ>

//where one output was put in test keys as the testing environment, and one generated a 'mysteriously' signed cert from
//a root ca cert/key pair that is not known (could be revoked or malicious)

func writeCorrectlySignedCert() {
	pem := Sign(testkeys.GetNodeCSRPath(), testkeys.GetGatewayCertPath(), testkeys.GetGatewayKeyPath())
	writeToFile(pem, testkeys.GetNodeCertPath_KnownSignature())
	fmt.Println("i did shit")
}

func TestSign(t *testing.T) {
	//Sign()
	writeCorrectlySignedCert()
}

//test repeatability by pulling the signed cert, resigning (they should be the same with the same csr, CACert
// and privKey
//Might be a hackey test since we're only going up to a certain length..Thoughts?
func TestSign_Consistency(t *testing.T) {
	expected := *getKnownSignature()
	clientCSR := loadCertificateRequest(testkeys.GetNodeCSRPath())
	fmt.Println("loaded CSR")
	caCert := loadCertificate(testkeys.GetGatewayCertPath())
	fmt.Println("loaded CA cert")
	caPrivKey := loadPrivKey(testkeys.GetGatewayKeyPath())
	fmt.Println("loaded CA key")
	observed := Sign(clientCSR, caCert, caPrivKey)

	fmt.Println(expected.Bytes)
	fmt.Println(observed)
	divison := 8
	fmt.Println(observed[:len(observed)/divison])
	//won't be exactly the same as some randomness is added..idiot
	if bytes.Compare(expected.Bytes[:len(expected.Bytes)/divison], observed[:len(observed)/divison]) != 0 {
		t.Error("Failed signature consistency")
	}
}

//Test the checksign is implemented correctly in sign
func TestSign_CheckSignature(t *testing.T) {
	Sign(testkeys.GetCertPath_PreviouslySignature(), testkeys.GetGatewayCertPath(), testkeys.GetGatewayKeyPath())

}

//put this in the ca.go file if it turns out to be more involved
func TestSign_VerifySignatureSuccess(t *testing.T) {
	rootCert := loadCertificate(testkeys.GetGatewayCertPath())
	signatureBytes := Sign(testkeys.GetNodeCSRPath(), testkeys.GetGatewayCertPath(), testkeys.GetGatewayKeyPath())
	signedCert, err := x509.ParseCertificate(signatureBytes)
	if err != nil {
		t.Error(err)
	}
	rootCert.Verify(signedCert.b)
}

//Check that an already signed cert does not pass
func TestSign_VerifySignatureFailure(t *testing.T) {

	alreadySignedCert := loadCertificate(testkeys.GetCertPath_PreviouslySignature())
	CACert := loadCertificate(testkeys.GetGatewayCertPath())

	err := alreadySignedCert.CheckSignatureFrom(CACert)

	if err == nil {
		t.Errorf("Failed to detect a certificate not signed by the root CA")
	}
}

//Test all the file opening things? Almost certainly a waste of time *shrugs*??
func Test_LoadCert(t *testing.T) {

}

func TestLoadCSR(t *testing.T) {

}

func TestLoadPrivKey(t *testing.T) {

}

/*
func TestSign_FileIsValidCert(t *testing.T) {

}
*/
