/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-config/configtx/membership"
	"github.com/hyperledger/fabric-config/configtx/orderer"
	"github.com/hyperledger/fabric-config/protolator"
	cb "github.com/hyperledger/fabric-protos-go/common"
	mb "github.com/hyperledger/fabric-protos-go/msp"
	. "github.com/onsi/gomega"
)

func TestApplicationMSP(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	expectedMSP, _ := baseMSP(t)

	application, _ := baseApplication(t)
	applicationGroup, err := newApplicationGroup(application)
	gt.Expect(err).NotTo(HaveOccurred())

	// We need to add the base MSP config to the base application since
	// newApplicationGroup doesn't apply MSP configuration
	applicationOrgGroup := applicationGroup.Groups["Org1"]
	fabricMSPConfig, err := expectedMSP.toProto()
	gt.Expect(err).NotTo(HaveOccurred())

	conf, err := proto.Marshal(fabricMSPConfig)
	gt.Expect(err).NotTo(HaveOccurred())

	mspConfig := &mb.MSPConfig{
		Config: conf,
	}

	err = setValue(applicationOrgGroup, mspValue(mspConfig), AdminsPolicyKey)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: &cb.ConfigGroup{
			Groups: map[string]*cb.ConfigGroup{
				ApplicationGroupKey: applicationGroup,
			},
		},
	}

	c := New(config)

	msp, err := c.Application().Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(msp).To(Equal(expectedMSP))
}

func TestOrdererMSP(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	soloOrderer, _ := baseSoloOrderer(t)
	expectedMSP := soloOrderer.Organizations[0].MSP

	ordererGroup, err := newOrdererGroup(soloOrderer)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: &cb.ConfigGroup{
			Groups: map[string]*cb.ConfigGroup{
				OrdererGroupKey: ordererGroup,
			},
		},
	}

	c := New(config)

	msp, err := c.Orderer().Organization("OrdererOrg").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(msp).To(Equal(expectedMSP))
}

func TestConsortiumMSP(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	consortiums, _ := baseConsortiums(t)
	expectedMSP := consortiums[0].Organizations[0].MSP

	consortiumsGroup, err := newConsortiumsGroup(consortiums)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: &cb.ConfigGroup{
			Groups: map[string]*cb.ConfigGroup{
				ConsortiumsGroupKey: consortiumsGroup,
			},
		},
	}

	c := New(config)

	msp, err := c.Consortium("Consortium1").Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(msp).To(Equal(expectedMSP))
}

func TestMSPConfigurationFailures(t *testing.T) {
	t.Parallel()

	badCert := &x509.Certificate{}

	tests := []struct {
		name           string
		orgType        string
		consortiumName string
		orgName        string
		mspMod         func(*MSP)
		expectedErr    string
	}{
		{
			name:    "Bad root cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				badCert := &x509.Certificate{}
				msp.RootCerts = append(msp.RootCerts, badCert)
			},
			expectedErr: "parsing root certs: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad intermediate cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.IntermediateCerts = append(msp.IntermediateCerts, badCert)
			},
			expectedErr: "parsing intermediate certs: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad admin cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.Admins = append(msp.Admins, badCert)
			},
			expectedErr: "parsing admin certs: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad OU Identifier cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.OrganizationalUnitIdentifiers[0].Certificate = badCert
			},
			expectedErr: "parsing ou identifiers: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad tls root cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.TLSRootCerts = append(msp.TLSRootCerts, badCert)
			},
			expectedErr: "parsing tls root certs: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad tls intermediate cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.TLSIntermediateCerts = append(msp.TLSIntermediateCerts, badCert)
			},
			expectedErr: "parsing tls intermediate certs: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad Client OU Identifier cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.NodeOUs.ClientOUIdentifier.Certificate = badCert
			},
			expectedErr: "parsing client ou identifier cert: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad Peer OU Identifier cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.NodeOUs.PeerOUIdentifier.Certificate = badCert
			},
			expectedErr: "parsing peer ou identifier cert: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad Admin OU Identifier cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.NodeOUs.AdminOUIdentifier.Certificate = badCert
			},
			expectedErr: "parsing admin ou identifier cert: asn1: syntax error: sequence truncated",
		},
		{
			name:    "Bad Orderer OU Identifier cert",
			orgType: OrdererGroupKey,
			orgName: "OrdererOrg",
			mspMod: func(msp *MSP) {
				msp.NodeOUs.OrdererOUIdentifier.Certificate = badCert
			},
			expectedErr: "parsing orderer ou identifier cert: asn1: syntax error: sequence truncated",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gt := NewGomegaWithT(t)

			consortiums, _ := baseConsortiums(t)
			consortiumsGroup, err := newConsortiumsGroup(consortiums)
			gt.Expect(err).NotTo(HaveOccurred())

			orderer, _ := baseSoloOrderer(t)
			ordererGroup, err := newOrdererGroup(orderer)
			gt.Expect(err).NotTo(HaveOccurred())

			application, _ := baseApplication(t)
			applicationGroup, err := newApplicationGroup(application)
			gt.Expect(err).NotTo(HaveOccurred())

			config := &cb.Config{
				ChannelGroup: &cb.ConfigGroup{
					Groups: map[string]*cb.ConfigGroup{
						ConsortiumsGroupKey: consortiumsGroup,
						OrdererGroupKey:     ordererGroup,
						ApplicationGroupKey: applicationGroup,
					},
				},
			}

			c := &ConfigTx{
				original: config,
				updated:  config,
			}
			if tt.mspMod != nil && tt.orgType != ConsortiumsGroupKey {
				baseMSP, _ := baseMSP(t)

				tt.mspMod(&baseMSP)

				orgGroup := c.updated.ChannelGroup.Groups[tt.orgType].Groups[tt.orgName]
				fabricMSPConfig, err := baseMSP.toProto()
				gt.Expect(err).NotTo(HaveOccurred())

				conf, err := proto.Marshal(fabricMSPConfig)
				gt.Expect(err).NotTo(HaveOccurred())

				mspConfig := &mb.MSPConfig{
					Config: conf,
				}

				err = setValue(orgGroup, mspValue(mspConfig), AdminsPolicyKey)
				gt.Expect(err).NotTo(HaveOccurred())
			}

			switch tt.orgType {
			case ApplicationGroupKey:
				_, err := c.Application().Organization(tt.orgName).MSP()
				gt.Expect(err).To(MatchError(tt.expectedErr))
			case OrdererGroupKey:
				_, err := c.Orderer().Organization(tt.orgName).MSP()
				gt.Expect(err).To(MatchError(tt.expectedErr))
			case ConsortiumsGroupKey:
				_, err := c.Consortium(tt.consortiumName).Organization(tt.orgName).MSP()
				gt.Expect(err).To(MatchError(tt.expectedErr))
			default:
				t.Fatalf("invalid org type %s", tt.orgType)
			}
		})
	}
}

func TestMSPToProto(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	msp, _ := baseMSP(t)
	certBase64, crlBase64 := certCRLBase64(t, msp)

	expectedFabricMSPConfigProtoJSON := fmt.Sprintf(`
{
	"admins": [
		"%[1]s"
	],
	"crypto_config": {
		"identity_identifier_hash_function": "SHA256",
		"signature_hash_family": "SHA3"
	},
	"fabric_node_ous": {
		"admin_ou_identifier": {
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		},
		"client_ou_identifier": {
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		},
		"enable": false,
		"orderer_ou_identifier": {
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		},
		"peer_ou_identifier": {
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		}
	},
	"intermediate_certs": [
		"%[1]s"
	],
	"name": "MSPID",
	"organizational_unit_identifiers": [
		{
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		}
	],
	"revocation_list": [
		"%[2]s"
	],
	"root_certs": [
		"%[1]s"
	],
	"signing_identity": null,
	"tls_intermediate_certs": [
		"%[1]s"
	],
	"tls_root_certs": [
		"%[1]s"
	]
}
`, certBase64, crlBase64)
	expectedFabricMSPConfigProto := &mb.FabricMSPConfig{}
	err := protolator.DeepUnmarshalJSON(bytes.NewBufferString(expectedFabricMSPConfigProtoJSON), expectedFabricMSPConfigProto)
	gt.Expect(err).NotTo(HaveOccurred())

	fabricMSPConfigProto, err := msp.toProto()
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(fabricMSPConfigProto).To(Equal(expectedFabricMSPConfigProto))
}

func TestMSPToProtoNoNodeOUs(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	msp, _ := baseMSP(t)
	msp.NodeOUs = membership.NodeOUs{}
	certBase64, crlBase64 := certCRLBase64(t, msp)

	expectedFabricMSPConfigProtoJSON := fmt.Sprintf(`
{
	"admins": [
		"%[1]s"
	],
	"crypto_config": {
		"identity_identifier_hash_function": "SHA256",
		"signature_hash_family": "SHA3"
	},
	"fabric_node_ous": null,
	"intermediate_certs": [
		"%[1]s"
	],
	"name": "MSPID",
	"organizational_unit_identifiers": [
		{
			"certificate": "%[1]s",
			"organizational_unit_identifier": "OUID"
		}
	],
	"revocation_list": [
		"%[2]s"
	],
	"root_certs": [
		"%[1]s"
	],
	"signing_identity": null,
	"tls_intermediate_certs": [
		"%[1]s"
	],
	"tls_root_certs": [
		"%[1]s"
	]
}
`, certBase64, crlBase64)
	expectedFabricMSPConfigProto := &mb.FabricMSPConfig{}
	err := protolator.DeepUnmarshalJSON(bytes.NewBufferString(expectedFabricMSPConfigProtoJSON), expectedFabricMSPConfigProto)
	gt.Expect(err).NotTo(HaveOccurred())

	fabricMSPConfigProto, err := msp.toProto()
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(fabricMSPConfigProto).To(Equal(expectedFabricMSPConfigProto))
}

func TestSetConsortiumMSP(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	consortiumGroup, privKeys, err := baseConsortiumChannelGroup(t)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: consortiumGroup,
	}
	c := New(config)

	consortium1 := c.Consortium("Consortium1")
	consortiumOrg1MSP, err := consortium1.Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	consortiumOrg2MSP, err := consortium1.Organization("Org2").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	consortiumOrg1CertBase64, consortiumOrg1CRLBase64 := certCRLBase64(t, consortiumOrg1MSP)
	consortiumOrg2CertBase64, consortiumOrg2CRLBase64 := certCRLBase64(t, consortiumOrg2MSP)

	newRootCert, newRootPrivKey := generateCACertAndPrivateKey(t, "anotherca-org1.example.com")
	newRootCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newRootCert))
	consortiumOrg1MSP.RootCerts = append(consortiumOrg1MSP.RootCerts, newRootCert)

	newIntermediateCert, _ := generateIntermediateCACertAndPrivateKey(t, "anotherca-org1.example.com", newRootCert, newRootPrivKey)
	newIntermediateCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newIntermediateCert))
	consortiumOrg1MSP.IntermediateCerts = append(consortiumOrg1MSP.IntermediateCerts, newIntermediateCert)

	cert := consortiumOrg1MSP.RootCerts[0]
	privKey := privKeys[0]
	certToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, "org1.example.com", cert, privKey)
	signingIdentity := &SigningIdentity{
		Certificate: cert,
		PrivateKey:  privKey,
		MSPID:       "MSPID",
	}
	newCRL, err := consortium1.Organization("Org1").CreateMSPCRL(signingIdentity, certToRevoke)
	gt.Expect(err).NotTo(HaveOccurred())
	pemNewCRL, err := pemEncodeCRL(newCRL)
	gt.Expect(err).NotTo(HaveOccurred())
	newCRLBase64 := base64.StdEncoding.EncodeToString(pemNewCRL)
	consortiumOrg1MSP.RevocationList = append(consortiumOrg1MSP.RevocationList, newCRL)

	err = consortium1.Organization("Org1").SetMSP(consortiumOrg1MSP)
	gt.Expect(err).NotTo(HaveOccurred())

	expectedConfigJSON := fmt.Sprintf(`
{
	"channel_group": {
		"groups": {
			"Consortiums": {
				"groups": {
					"Consortium1": {
						"groups": {
							"Org1": {
								"groups": {},
								"mod_policy": "Admins",
								"policies": {
									"Admins": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "MAJORITY",
												"sub_policy": "Admins"
											}
										},
										"version": "0"
									},
									"Endorsement": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "MAJORITY",
												"sub_policy": "Endorsement"
											}
										},
										"version": "0"
									},
									"Readers": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "ANY",
												"sub_policy": "Readers"
											}
										},
										"version": "0"
									},
									"Writers": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "ANY",
												"sub_policy": "Writers"
											}
										},
										"version": "0"
									}
								},
								"values": {
									"MSP": {
										"mod_policy": "Admins",
										"value": {
											"config": {
												"admins": [
													"%[1]s"
												],
												"crypto_config": {
													"identity_identifier_hash_function": "SHA256",
													"signature_hash_family": "SHA3"
												},
												"fabric_node_ous": {
													"admin_ou_identifier": {
														"certificate": "%[1]s",
														"organizational_unit_identifier": "OUID"
													},
													"client_ou_identifier": {
														"certificate": "%[1]s",
														"organizational_unit_identifier": "OUID"
													},
													"enable": false,
													"orderer_ou_identifier": {
														"certificate": "%[1]s",
														"organizational_unit_identifier": "OUID"
													},
													"peer_ou_identifier": {
														"certificate": "%[1]s",
														"organizational_unit_identifier": "OUID"
													}
												},
												"intermediate_certs": [
													"%[1]s",
													"%[2]s"
												],
												"name": "MSPID",
												"organizational_unit_identifiers": [
													{
														"certificate": "%[1]s",
														"organizational_unit_identifier": "OUID"
													}
												],
												"revocation_list": [
													"%[3]s",
													"%[4]s"
												],
												"root_certs": [
													"%[1]s",
													"%[5]s"
												],
												"signing_identity": null,
												"tls_intermediate_certs": [
													"%[1]s"
												],
												"tls_root_certs": [
													"%[1]s"
												]
											},
											"type": 0
										},
										"version": "0"
									}
								},
								"version": "0"
							},
							"Org2": {
								"groups": {},
								"mod_policy": "Admins",
								"policies": {
									"Admins": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "MAJORITY",
												"sub_policy": "Admins"
											}
										},
										"version": "0"
									},
									"Endorsement": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "MAJORITY",
												"sub_policy": "Endorsement"
											}
										},
										"version": "0"
									},
									"Readers": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "ANY",
												"sub_policy": "Readers"
											}
										},
										"version": "0"
									},
									"Writers": {
										"mod_policy": "Admins",
										"policy": {
											"type": 3,
											"value": {
												"rule": "ANY",
												"sub_policy": "Writers"
											}
										},
										"version": "0"
									}
								},
								"values": {
									"MSP": {
										"mod_policy": "Admins",
										"value": {
											"config": {
												"admins": [
													"%[6]s"
												],
												"crypto_config": {
													"identity_identifier_hash_function": "SHA256",
													"signature_hash_family": "SHA3"
												},
												"fabric_node_ous": {
													"admin_ou_identifier": {
														"certificate": "%[6]s",
														"organizational_unit_identifier": "OUID"
													},
													"client_ou_identifier": {
														"certificate": "%[6]s",
														"organizational_unit_identifier": "OUID"
													},
													"enable": false,
													"orderer_ou_identifier": {
														"certificate": "%[6]s",
														"organizational_unit_identifier": "OUID"
													},
													"peer_ou_identifier": {
														"certificate": "%[6]s",
														"organizational_unit_identifier": "OUID"
													}
												},
												"intermediate_certs": [
													"%[6]s"
												],
												"name": "MSPID",
												"organizational_unit_identifiers": [
													{
														"certificate": "%[6]s",
														"organizational_unit_identifier": "OUID"
													}
												],
												"revocation_list": [
													"%[7]s"
												],
												"root_certs": [
													"%[6]s"
												],
												"signing_identity": null,
												"tls_intermediate_certs": [
													"%[6]s"
												],
												"tls_root_certs": [
													"%[6]s"
												]
											},
											"type": 0
										},
										"version": "0"
									}
								},
								"version": "0"
							}
						},
						"mod_policy": "/Channel/Orderer/Admins",
						"policies": {},
						"values": {
							"ChannelCreationPolicy": {
								"mod_policy": "/Channel/Orderer/Admins",
								"value": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Admins"
									}
								},
								"version": "0"
							}
						},
						"version": "0"
					}
				},
				"mod_policy": "/Channel/Orderer/Admins",
				"policies": {
					"Admins": {
						"mod_policy": "/Channel/Orderer/Admins",
						"policy": {
							"type": 1,
							"value": {
								"identities": [],
								"rule": {
									"n_out_of": {
										"n": 0,
										"rules": []
									}
								},
								"version": 0
							}
						},
						"version": "0"
					}
				},
				"values": {},
				"version": "0"
			}
		},
		"mod_policy": "",
		"policies": {},
		"values": {},
		"version": "0"
	},
	"sequence": "0"
}
`, consortiumOrg1CertBase64, newIntermediateCertBase64, consortiumOrg1CRLBase64, newCRLBase64, newRootCertBase64, consortiumOrg2CertBase64, consortiumOrg2CRLBase64)

	buf := bytes.Buffer{}
	err = protolator.DeepMarshalJSON(&buf, c.updated)
	gt.Expect(err).NotTo(HaveOccurred())

	gt.Expect(buf.String()).To(MatchJSON(expectedConfigJSON))
}

func TestSetConsortiumMSPFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		spec           string
		mspMod         func(MSP) MSP
		consortiumName string
		orgName        string
		expectedErr    string
	}{
		{
			spec: "updating msp name",
			mspMod: func(msp MSP) MSP {
				msp.Name = "thiscantbegood"
				return msp
			},
			consortiumName: "Consortium1",
			orgName:        "Org1",
			expectedErr:    "MSP name cannot be changed",
		},
		{
			spec: "invalid root ca cert keyusage",
			mspMod: func(msp MSP) MSP {
				msp.RootCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageKeyAgreement,
					},
				}
				return msp
			},
			consortiumName: "Consortium1",
			orgName:        "Org1",
			expectedErr:    "invalid root cert: KeyUsage must be x509.KeyUsageCertSign. serial number: 7",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)

			consortiumGroup, _, err := baseConsortiumChannelGroup(t)
			gt.Expect(err).NotTo(HaveOccurred())

			config := &cb.Config{
				ChannelGroup: consortiumGroup,
			}
			c := New(config)

			consortiumOrg1 := c.Consortium("Consortium1").Organization("Org1")
			consortiumOrg1MSP, err := consortiumOrg1.MSP()
			gt.Expect(err).NotTo(HaveOccurred())

			consortiumOrg1MSP = tc.mspMod(consortiumOrg1MSP)
			err = consortiumOrg1.SetMSP(consortiumOrg1MSP)
			gt.Expect(err).To(MatchError(tc.expectedErr))
		})
	}
}

func TestUpdateOrdererMSP(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	channelGroup, privKeys, err := baseOrdererChannelGroup(t, orderer.ConsensusTypeSolo)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: channelGroup,
	}
	c := New(config)

	ordererMSP, err := c.Orderer().Organization("OrdererOrg").MSP()
	gt.Expect(err).NotTo(HaveOccurred())

	ordererCertBase64, ordererCRLBase64 := certCRLBase64(t, ordererMSP)

	newRootCert, newRootPrivKey := generateCACertAndPrivateKey(t, "anotherca-org1.example.com")
	newRootCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newRootCert))
	ordererMSP.RootCerts = append(ordererMSP.RootCerts, newRootCert)

	newIntermediateCert, _ := generateIntermediateCACertAndPrivateKey(t, "anotherca-org1.example.com", newRootCert, newRootPrivKey)
	newIntermediateCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newIntermediateCert))
	ordererMSP.IntermediateCerts = append(ordererMSP.IntermediateCerts, newIntermediateCert)

	cert := ordererMSP.RootCerts[0]
	certToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, "org1.example.com", cert, privKeys[0])
	signingIdentity := &SigningIdentity{
		Certificate: cert,
		PrivateKey:  privKeys[0],
		MSPID:       "MSPID",
	}
	newCRL, err := c.Orderer().Organization("OrdererOrg").CreateMSPCRL(signingIdentity, certToRevoke)
	gt.Expect(err).NotTo(HaveOccurred())
	pemNewCRL, err := pemEncodeCRL(newCRL)
	gt.Expect(err).NotTo(HaveOccurred())
	newCRLBase64 := base64.StdEncoding.EncodeToString(pemNewCRL)
	ordererMSP.RevocationList = append(ordererMSP.RevocationList, newCRL)

	err = c.Orderer().Organization("OrdererOrg").SetMSP(ordererMSP)
	gt.Expect(err).NotTo(HaveOccurred())

	expectedConfigJSON := fmt.Sprintf(`
{
	"channel_group": {
		"groups": {
			"Orderer": {
				"groups": {
					"OrdererOrg": {
						"groups": {},
						"mod_policy": "Admins",
						"policies": {
							"Admins": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Admins"
									}
								},
								"version": "0"
							},
							"Endorsement": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Endorsement"
									}
								},
								"version": "0"
							},
							"Readers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Readers"
									}
								},
								"version": "0"
							},
							"Writers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Writers"
									}
								},
								"version": "0"
							}
						},
						"values": {
							"Endpoints": {
								"mod_policy": "Admins",
								"value": {
									"addresses": [
										"localhost:123"
									]
								},
								"version": "0"
							},
							"MSP": {
								"mod_policy": "Admins",
								"value": {
									"config": {
										"admins": [
											"%[1]s"
										],
										"crypto_config": {
											"identity_identifier_hash_function": "SHA256",
											"signature_hash_family": "SHA3"
										},
										"fabric_node_ous": {
											"admin_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"client_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"enable": false,
											"orderer_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"peer_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											}
										},
										"intermediate_certs": [
											"%[1]s",
											"%[2]s"
										],
										"name": "MSPID",
										"organizational_unit_identifiers": [
											{
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											}
										],
										"revocation_list": [
											"%[3]s",
											"%[4]s"
										],
										"root_certs": [
											"%[1]s",
											"%[5]s"
										],
										"signing_identity": null,
										"tls_intermediate_certs": [
											"%[1]s"
										],
										"tls_root_certs": [
											"%[1]s"
										]
									},
									"type": 0
								},
								"version": "0"
							}
						},
						"version": "0"
					}
				},
				"mod_policy": "Admins",
				"policies": {
					"Admins": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "MAJORITY",
								"sub_policy": "Admins"
							}
						},
						"version": "0"
					},
					"BlockValidation": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "ANY",
								"sub_policy": "Writers"
							}
						},
						"version": "0"
					},
					"Readers": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "ANY",
								"sub_policy": "Readers"
							}
						},
						"version": "0"
					},
					"Writers": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "ANY",
								"sub_policy": "Writers"
							}
						},
						"version": "0"
					}
				},
				"values": {
					"BatchSize": {
						"mod_policy": "Admins",
						"value": {
							"absolute_max_bytes": 100,
							"max_message_count": 100,
							"preferred_max_bytes": 100
						},
						"version": "0"
					},
					"BatchTimeout": {
						"mod_policy": "Admins",
						"value": {
							"timeout": "0s"
						},
						"version": "0"
					},
					"Capabilities": {
						"mod_policy": "Admins",
						"value": {
							"capabilities": {
								"V1_3": {}
							}
						},
						"version": "0"
					},
					"ChannelRestrictions": {
						"mod_policy": "Admins",
						"value": null,
						"version": "0"
					},
					"ConsensusType": {
						"mod_policy": "Admins",
						"value": {
							"metadata": null,
							"state": "STATE_NORMAL",
							"type": "solo"
						},
						"version": "0"
					}
				},
				"version": "0"
			}
		},
		"mod_policy": "",
		"policies": {},
		"values": {},
		"version": "0"
	},
	"sequence": "0"
}`, ordererCertBase64, newIntermediateCertBase64, ordererCRLBase64, newCRLBase64, newRootCertBase64)

	buf := bytes.Buffer{}
	err = protolator.DeepMarshalJSON(&buf, c.updated)
	gt.Expect(err).NotTo(HaveOccurred())

	gt.Expect(buf.String()).To(MatchJSON(expectedConfigJSON))
}

func TestUpdateOrdererMSPFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		spec        string
		mspMod      func(MSP) MSP
		orgName     string
		expectedErr string
	}{
		{
			spec: "updating msp name",
			mspMod: func(msp MSP) MSP {
				msp.Name = "thiscantbegood"
				return msp
			},
			orgName:     "OrdererOrg",
			expectedErr: "MSP name cannot be changed",
		},
		{
			spec: "invalid root ca cert keyusage",
			mspMod: func(msp MSP) MSP {
				msp.RootCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageKeyAgreement,
					},
				}
				return msp
			},
			orgName:     "OrdererOrg",
			expectedErr: "invalid root cert: KeyUsage must be x509.KeyUsageCertSign. serial number: 7",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)

			channelGroup, _, err := baseOrdererChannelGroup(t, orderer.ConsensusTypeSolo)
			gt.Expect(err).NotTo(HaveOccurred())

			config := &cb.Config{
				ChannelGroup: channelGroup,
			}
			c := New(config)

			ordererMSP, err := c.Orderer().Organization("OrdererOrg").MSP()
			gt.Expect(err).NotTo(HaveOccurred())

			ordererMSP = tc.mspMod(ordererMSP)
			err = c.Orderer().Organization(tc.orgName).SetMSP(ordererMSP)
			gt.Expect(err).To(MatchError(tc.expectedErr))
		})
	}
}

func TestSetApplicationMSP(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	channelGroup, privateKeys, err := baseApplicationChannelGroup(t)
	gt.Expect(err).ToNot(HaveOccurred())
	config := &cb.Config{
		ChannelGroup: channelGroup,
	}

	c := New(config)

	org1MSP, err := c.Application().Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org2MSP, err := c.Application().Organization("Org2").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org1CertBase64, org1CRLBase64 := certCRLBase64(t, org1MSP)
	org2CertBase64, org2CRLBase64 := certCRLBase64(t, org2MSP)

	newRootCert, newRootPrivKey := generateCACertAndPrivateKey(t, "anotherca-org1.example.com")
	newRootCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newRootCert))
	org1MSP.RootCerts = append(org1MSP.RootCerts, newRootCert)

	newIntermediateCert, _ := generateIntermediateCACertAndPrivateKey(t, "anotherca-org1.example.com", newRootCert, newRootPrivKey)
	newIntermediateCertBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(newIntermediateCert))
	org1MSP.IntermediateCerts = append(org1MSP.IntermediateCerts, newIntermediateCert)

	cert := org1MSP.RootCerts[0]
	privKey := privateKeys[0]
	certToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, "org1.example.com", cert, privKey)
	signingIdentity := &SigningIdentity{
		Certificate: cert,
		PrivateKey:  privKey,
		MSPID:       "MSPID",
	}
	newCRL, err := c.Application().Organization("Org1").CreateMSPCRL(signingIdentity, certToRevoke)
	gt.Expect(err).NotTo(HaveOccurred())
	pemNewCRL, err := pemEncodeCRL(newCRL)
	gt.Expect(err).NotTo(HaveOccurred())
	newCRLBase64 := base64.StdEncoding.EncodeToString(pemNewCRL)
	org1MSP.RevocationList = append(org1MSP.RevocationList, newCRL)

	err = c.Application().Organization("Org1").SetMSP(org1MSP)
	gt.Expect(err).NotTo(HaveOccurred())

	expectedConfigJSON := fmt.Sprintf(`
{
	"channel_group": {
		"groups": {
			"Application": {
				"groups": {
					"Org1": {
						"groups": {},
						"mod_policy": "Admins",
						"policies": {
							"Admins": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Admins"
									}
								},
								"version": "0"
							},
							"Endorsement": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Endorsement"
									}
								},
								"version": "0"
							},
							"LifecycleEndorsement": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Endorsement"
									}
								},
								"version": "0"
							},
							"Readers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Readers"
									}
								},
								"version": "0"
							},
							"Writers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Writers"
									}
								},
								"version": "0"
							}
						},
						"values": {
							"MSP": {
								"mod_policy": "Admins",
								"value": {
									"config": {
										"admins": [
											"%[1]s"
										],
										"crypto_config": {
											"identity_identifier_hash_function": "SHA256",
											"signature_hash_family": "SHA3"
										},
										"fabric_node_ous": {
											"admin_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"client_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"enable": false,
											"orderer_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											},
											"peer_ou_identifier": {
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											}
										},
										"intermediate_certs": [
											"%[1]s",
											"%[2]s"
										],
										"name": "MSPID",
										"organizational_unit_identifiers": [
											{
												"certificate": "%[1]s",
												"organizational_unit_identifier": "OUID"
											}
										],
										"revocation_list": [
											"%[3]s",
											"%[4]s"
										],
										"root_certs": [
											"%[1]s",
											"%[5]s"
										],
										"signing_identity": null,
										"tls_intermediate_certs": [
											"%[1]s"
										],
										"tls_root_certs": [
											"%[1]s"
										]
									},
									"type": 0
								},
								"version": "0"
							}
						},
						"version": "0"
					},
					"Org2": {
						"groups": {},
						"mod_policy": "Admins",
						"policies": {
							"Admins": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Admins"
									}
								},
								"version": "0"
							},
							"Endorsement": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Endorsement"
									}
								},
								"version": "0"
							},
							"LifecycleEndorsement": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "MAJORITY",
										"sub_policy": "Endorsement"
									}
								},
								"version": "0"
							},
							"Readers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Readers"
									}
								},
								"version": "0"
							},
							"Writers": {
								"mod_policy": "Admins",
								"policy": {
									"type": 3,
									"value": {
										"rule": "ANY",
										"sub_policy": "Writers"
									}
								},
								"version": "0"
							}
						},
						"values": {
							"MSP": {
								"mod_policy": "Admins",
								"value": {
									"config": {
										"admins": [
											"%[6]s"
										],
										"crypto_config": {
											"identity_identifier_hash_function": "SHA256",
											"signature_hash_family": "SHA3"
										},
										"fabric_node_ous": {
											"admin_ou_identifier": {
												"certificate": "%[6]s",
												"organizational_unit_identifier": "OUID"
											},
											"client_ou_identifier": {
												"certificate": "%[6]s",
												"organizational_unit_identifier": "OUID"
											},
											"enable": false,
											"orderer_ou_identifier": {
												"certificate": "%[6]s",
												"organizational_unit_identifier": "OUID"
											},
											"peer_ou_identifier": {
												"certificate": "%[6]s",
												"organizational_unit_identifier": "OUID"
											}
										},
										"intermediate_certs": [
											"%[6]s"
										],
										"name": "MSPID",
										"organizational_unit_identifiers": [
											{
												"certificate": "%[6]s",
												"organizational_unit_identifier": "OUID"
											}
										],
										"revocation_list": [
											"%[7]s"
										],
										"root_certs": [
											"%[6]s"
										],
										"signing_identity": null,
										"tls_intermediate_certs": [
											"%[6]s"
										],
										"tls_root_certs": [
											"%[6]s"
										]
									},
									"type": 0
								},
								"version": "0"
							}
						},
						"version": "0"
					}
				},
				"mod_policy": "Admins",
				"policies": {
					"Admins": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "MAJORITY",
								"sub_policy": "Admins"
							}
						},
						"version": "0"
					},
					"Readers": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "ANY",
								"sub_policy": "Readers"
							}
						},
						"version": "0"
					},
					"Writers": {
						"mod_policy": "Admins",
						"policy": {
							"type": 3,
							"value": {
								"rule": "ANY",
								"sub_policy": "Writers"
							}
						},
						"version": "0"
					}
				},
				"values": {
					"ACLs": {
						"mod_policy": "Admins",
						"value": {
							"acls": {
								"acl1": {
									"policy_ref": "hi"
								}
							}
						},
						"version": "0"
					},
					"Capabilities": {
						"mod_policy": "Admins",
						"value": {
							"capabilities": {
								"V1_3": {}
							}
						},
						"version": "0"
					}
				},
				"version": "0"
			}
		},
		"mod_policy": "",
		"policies": {},
		"values": {},
		"version": "0"
	},
	"sequence": "0"
}
`, org1CertBase64, newIntermediateCertBase64, org1CRLBase64, newCRLBase64, newRootCertBase64, org2CertBase64, org2CRLBase64)

	buf := bytes.Buffer{}
	err = protolator.DeepMarshalJSON(&buf, c.updated)
	gt.Expect(err).NotTo(HaveOccurred())

	gt.Expect(buf.String()).To(MatchJSON(expectedConfigJSON))
}

func TestSetApplicationMSPFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		spec        string
		mspMod      func(MSP) MSP
		orgName     string
		expectedErr string
	}{
		{
			spec: "updating msp name",
			mspMod: func(msp MSP) MSP {
				msp.Name = "thiscantbegood"
				return msp
			},
			orgName:     "Org1",
			expectedErr: "MSP name cannot be changed",
		},
		{
			spec: "invalid root ca cert keyusage",
			mspMod: func(msp MSP) MSP {
				msp.RootCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageKeyAgreement,
					},
				}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "invalid root cert: KeyUsage must be x509.KeyUsageCertSign. serial number: 7",
		},
		{
			spec: "root ca cert is not a ca",
			mspMod: func(msp MSP) MSP {
				msp.RootCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageCertSign,
						IsCA:         false,
					},
				}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "invalid root cert: must be a CA certificate. serial number: 7",
		},
		{
			spec: "invalid intermediate ca keyusage",
			mspMod: func(msp MSP) MSP {
				msp.IntermediateCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageKeyAgreement,
					},
				}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "invalid intermediate cert: KeyUsage must be x509.KeyUsageCertSign. serial number: 7",
		},
		{
			spec: "invalid intermediate cert -- not signed by root cert",
			mspMod: func(msp MSP) MSP {
				cert, _ := generateCACertAndPrivateKey(t, "org1.example.com")
				cert.SerialNumber = big.NewInt(7)
				msp.IntermediateCerts = []*x509.Certificate{cert}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "intermediate cert not signed by any root certs of this MSP. serial number: 7",
		},
		{
			spec: "tls root ca cert is not a ca",
			mspMod: func(msp MSP) MSP {
				msp.TLSRootCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageCertSign,
						IsCA:         false,
					},
				}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "invalid tls root cert: must be a CA certificate. serial number: 7",
		},
		{
			spec: "tls intemediate ca cert is not a ca",
			mspMod: func(msp MSP) MSP {
				msp.TLSIntermediateCerts = []*x509.Certificate{
					{
						SerialNumber: big.NewInt(7),
						KeyUsage:     x509.KeyUsageCertSign,
						IsCA:         false,
					},
				}
				return msp
			},
			orgName:     "Org1",
			expectedErr: "invalid tls intermediate cert: must be a CA certificate. serial number: 7",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)
			channelGroup, _, err := baseApplicationChannelGroup(t)
			gt.Expect(err).ToNot(HaveOccurred())
			config := &cb.Config{
				ChannelGroup: channelGroup,
			}

			c := New(config)

			org1MSP, err := c.Application().Organization("Org1").MSP()
			gt.Expect(err).NotTo(HaveOccurred())

			org1MSP = tc.mspMod(org1MSP)
			err = c.Application().Organization(tc.orgName).SetMSP(org1MSP)
			gt.Expect(err).To(MatchError(tc.expectedErr))
		})
	}
}

func TestCreateApplicationMSPCRL(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	channelGroup, privKeys, err := baseApplicationChannelGroup(t)
	gt.Expect(err).ToNot(HaveOccurred())
	config := &cb.Config{
		ChannelGroup: channelGroup,
	}

	c := New(config)

	org1MSP, err := c.Application().Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org1RootCert := org1MSP.RootCerts[0]
	org1PrivKey := privKeys[0]

	// update org2MSP to include an intemediate cert that is different
	// from the root cert
	org2MSP, err := c.Application().Organization("Org2").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org2Cert := org2MSP.RootCerts[0]
	org2PrivKey := privKeys[1]
	org2IntermediateCert, org2IntermediatePrivKey := generateIntermediateCACertAndPrivateKey(t, "org2.example.com", org2Cert, org2PrivKey)
	org2MSP.IntermediateCerts = append(org2MSP.IntermediateCerts, org2IntermediateCert)
	err = c.Application().Organization("Org2").SetMSP(org2MSP)
	gt.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		spec             string
		orgName          string
		caCert           *x509.Certificate
		caPrivKey        *ecdsa.PrivateKey
		numCertsToRevoke int
	}{
		{
			spec:             "create CRL using a root cert",
			orgName:          "Org1",
			caCert:           org1RootCert,
			caPrivKey:        org1PrivKey,
			numCertsToRevoke: 2,
		},
		{
			spec:             "create CRL using an intermediate cert",
			orgName:          "Org2",
			caCert:           org2IntermediateCert,
			caPrivKey:        org2IntermediatePrivKey,
			numCertsToRevoke: 1,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)
			certsToRevoke := make([]*x509.Certificate, tc.numCertsToRevoke)
			for i := 0; i < tc.numCertsToRevoke; i++ {
				certToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, tc.orgName, tc.caCert, tc.caPrivKey)
				certsToRevoke[i] = certToRevoke
			}
			signingIdentity := &SigningIdentity{
				Certificate: tc.caCert,
				PrivateKey:  tc.caPrivKey,
				MSPID:       "MSPID",
			}
			crl, err := c.Application().Organization(tc.orgName).CreateMSPCRL(signingIdentity, certsToRevoke...)
			gt.Expect(err).NotTo(HaveOccurred())
			err = tc.caCert.CheckCRLSignature(crl)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(crl.TBSCertList.RevokedCertificates).To(HaveLen(tc.numCertsToRevoke))
			for i := 0; i < tc.numCertsToRevoke; i++ {
				gt.Expect(crl.TBSCertList.RevokedCertificates[i].SerialNumber).To(Equal(certsToRevoke[i].SerialNumber))
			}
		})
	}
}

func TestCreateApplicationMSPCRLFailure(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	channelGroup, privKeys, err := baseApplicationChannelGroup(t)
	gt.Expect(err).ToNot(HaveOccurred())
	config := &cb.Config{
		ChannelGroup: channelGroup,
	}

	c := New(config)

	org1MSP, err := c.Application().Organization("Org1").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org1Cert := org1MSP.RootCerts[0]
	org1PrivKey := privKeys[0]
	org1CertToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, "org1.example.com", org1Cert, org1PrivKey)

	org2MSP, err := c.Application().Organization("Org2").MSP()
	gt.Expect(err).NotTo(HaveOccurred())
	org2Cert := org2MSP.RootCerts[0]
	org2PrivKey := privKeys[1]
	org2CertToRevoke, _ := generateCertAndPrivateKeyFromCACert(t, "org2.example.com", org2Cert, org2PrivKey)

	signingIdentity := &SigningIdentity{
		Certificate: org1Cert,
		PrivateKey:  org1PrivKey,
	}
	tests := []struct {
		spec            string
		mspMod          func(MSP) MSP
		signingIdentity *SigningIdentity
		certToRevoke    *x509.Certificate
		orgName         string
		expectedErr     string
	}{
		{
			spec:    "signing cert is not a root/intermediate cert for msp",
			orgName: "Org1",
			signingIdentity: &SigningIdentity{
				Certificate: org2Cert,
				PrivateKey:  org2PrivKey,
			},
			certToRevoke: org1CertToRevoke,
			expectedErr:  "signing cert is not a root/intermediate cert for this MSP: MSPID",
		},
		{
			spec:            "certificate not issued by this MSP",
			orgName:         "Org1",
			signingIdentity: signingIdentity,
			certToRevoke:    org2CertToRevoke,
			expectedErr:     fmt.Sprintf("certificate not issued by this MSP. serial number: %d", org2CertToRevoke.SerialNumber),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)

			newCRL, err := c.Application().Organization(tc.orgName).CreateMSPCRL(tc.signingIdentity, tc.certToRevoke)
			gt.Expect(err).To(MatchError(tc.expectedErr))
			gt.Expect(newCRL).To(BeNil())
		})
	}
}

func TestParseCertificateFromBytesFailure(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	errCert := `
-----END CERTIFICATE-----
`

	_, err := parseCertificateFromBytes([]byte(errCert))
	gt.Expect(err).NotTo(BeNil())
	gt.Expect(err.Error()).To(ContainSubstring("no PEM data found in cert["))

	_, err = parseCertificateFromBytes(nil)
	gt.Expect(err).To(MatchError("no PEM data found in cert[]"))
}

func TestParseCRLFailure(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	errCRL := `
-----END X509 CRL-----
`

	_, err := parseCRL([][]byte{[]byte(errCRL)})
	gt.Expect(err).NotTo(BeNil())
	gt.Expect(err.Error()).To(ContainSubstring("no PEM data found in CRL["))

	_, err = parseCRL([][]byte{nil, []byte(errCRL)})
	gt.Expect(err).To(MatchError("no PEM data found in CRL[]"))
}

func TestParsePrivateKeyFromBytesFailure(t *testing.T) {
	t.Parallel()
	gt := NewGomegaWithT(t)

	errPrivateKey := `
-----END EC PRIVATE KEY-----
`

	_, err := parsePrivateKeyFromBytes([]byte(errPrivateKey))
	gt.Expect(err).NotTo(BeNil())
	gt.Expect(err.Error()).To(ContainSubstring("no PEM data found in private key["))
}

func baseMSP(t *testing.T) (MSP, *ecdsa.PrivateKey) {
	gt := NewGomegaWithT(t)

	cert, privKey := generateCACertAndPrivateKey(t, "org1.example.com")
	crlBytes, err := cert.CreateCRL(rand.Reader, privKey, nil, time.Now(), time.Now().Add(YEAR))
	gt.Expect(err).NotTo(HaveOccurred())

	crl, err := x509.ParseCRL(crlBytes)
	gt.Expect(err).NotTo(HaveOccurred())

	return MSP{
		Name:              "MSPID",
		RootCerts:         []*x509.Certificate{cert},
		IntermediateCerts: []*x509.Certificate{cert},
		Admins:            []*x509.Certificate{cert},
		RevocationList:    []*pkix.CertificateList{crl},
		OrganizationalUnitIdentifiers: []membership.OUIdentifier{
			{
				Certificate:                  cert,
				OrganizationalUnitIdentifier: "OUID",
			},
		},
		CryptoConfig: membership.CryptoConfig{
			SignatureHashFamily:            "SHA3",
			IdentityIdentifierHashFunction: "SHA256",
		},
		TLSRootCerts:         []*x509.Certificate{cert},
		TLSIntermediateCerts: []*x509.Certificate{cert},
		NodeOUs: membership.NodeOUs{
			ClientOUIdentifier: membership.OUIdentifier{
				Certificate:                  cert,
				OrganizationalUnitIdentifier: "OUID",
			},
			PeerOUIdentifier: membership.OUIdentifier{
				Certificate:                  cert,
				OrganizationalUnitIdentifier: "OUID",
			},
			AdminOUIdentifier: membership.OUIdentifier{
				Certificate:                  cert,
				OrganizationalUnitIdentifier: "OUID",
			},
			OrdererOUIdentifier: membership.OUIdentifier{
				Certificate:                  cert,
				OrganizationalUnitIdentifier: "OUID",
			},
		},
	}, privKey
}

// certCRLBase64 returns a base64 encoded representation of
// the first root certificate, the private key, and the first revocation list
// for the specified MSP. These are intended for use when formatting the
// expected config in JSON format.
func certCRLBase64(t *testing.T, msp MSP) (string, string) {
	gt := NewGomegaWithT(t)

	cert := msp.RootCerts[0]
	crl := msp.RevocationList[0]

	certBase64 := base64.StdEncoding.EncodeToString(pemEncodeX509Certificate(cert))
	pemCRLBytes, err := buildPemEncodedRevocationList([]*pkix.CertificateList{crl})
	gt.Expect(err).NotTo(HaveOccurred())
	crlBase64 := base64.StdEncoding.EncodeToString(pemCRLBytes[0])

	return certBase64, crlBase64

}
