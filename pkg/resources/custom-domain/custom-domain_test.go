package custom_domain

import (
	"context"
	"github.com/integr8ly/integreatly-operator/apis/v1alpha1"
	customdomainv1alpha1 "github.com/openshift/custom-domains-operator/api/v1alpha1"
	olm "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestGetDomain(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = olm.AddToScheme(scheme)

	exampleNameSpace := "redhat-rhoam-operator"

	rhoamInstallation := &v1alpha1.RHMI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-api",
			Namespace: exampleNameSpace,
		},
		Spec: v1alpha1.RHMISpec{
			Type: "managed-api",
		},
	}

	type args struct {
		ctx          context.Context
		client       client.Client
		installation *v1alpha1.RHMI
	}
	tests := []struct {
		name    string
		args    args
		ok      bool
		domain  string
		wantErr bool
	}{
		{
			name: "Custom Domain successfully gotten",
			args: args{
				ctx: context.TODO(),
				client: fake.NewFakeClientWithScheme(scheme,
					&corev1.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name:      "addon-managed-api-service-parameters",
							Namespace: exampleNameSpace,
						},
						Data: map[string][]byte{
							"custom-domain_domain": []byte("apps.example.com"),
						},
					},
				),
				installation: rhoamInstallation,
			},
			ok:      true,
			domain:  "apps.example.com",
			wantErr: false,
		},
		{
			name: "Custom Domain has leading/trailing white spaces",
			args: args{
				ctx: context.TODO(),
				client: fake.NewFakeClientWithScheme(scheme,
					&corev1.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name:      "addon-managed-api-service-parameters",
							Namespace: exampleNameSpace,
						},
						Data: map[string][]byte{
							"custom-domain_domain": []byte("  apps.example.com  "),
						},
					}),
				installation: rhoamInstallation,
			},
			ok:      true,
			domain:  "apps.example.com",
			wantErr: false,
		},
		{
			name: "No Custom Domain set in addon secret",
			args: args{
				ctx: context.TODO(),
				client: fake.NewFakeClientWithScheme(scheme,
					&corev1.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name:      "addon-managed-api-service-parameters",
							Namespace: exampleNameSpace,
						},
						Data: map[string][]byte{},
					}),
				installation: rhoamInstallation,
			},
			ok:      false,
			domain:  "",
			wantErr: false,
		},
		{
			name: "Invalid Custom Domain set in addon secret",
			args: args{
				ctx: context.TODO(),
				client: fake.NewFakeClientWithScheme(scheme,
					&corev1.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name:      "addon-managed-api-service-parameters",
							Namespace: exampleNameSpace,
						},
						Data: map[string][]byte{
							"custom-domain_domain": []byte("bad domain"),
						},
					}),
				installation: rhoamInstallation,
			},
			ok:      true,
			domain:  "bad domain",
			wantErr: true,
		},
		{
			name: "Error getting addon secret",
			args: args{
				ctx: context.TODO(),
				client: fake.NewFakeClientWithScheme(scheme,
					&corev1.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name:      "addon-dummy-service-parameters",
							Namespace: exampleNameSpace,
						},
						Data: map[string][]byte{},
					}),
				installation: rhoamInstallation,
			},
			ok:      false,
			domain:  "",
			wantErr: true,
		},
		{
			name:    "Nil pointer passed in for installation type",
			ok:      false,
			domain:  "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, domain, err := GetDomain(tt.args.ctx, tt.args.client, tt.args.installation)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDomain() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.ok {
				t.Errorf("GetDomain() ok = %v, wanted ok = %v", ok, tt.ok)
			}
			if domain != tt.domain {
				t.Errorf("GetDomain() domain = %v, wanted domain = %v", domain, tt.domain)
			}
		})
	}
}

func TestIsValidDomain(t *testing.T) {
	type args struct {
		domain string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid domain",
			args: args{domain: "good.domain.com"},
			want: true,
		},
		{
			name: "Invalid domain",
			args: args{domain: "bad domain.com"},
			want: false,
		},
		{
			name: "Domain with unwanted prefix",
			args: args{domain: "https://prefix.domain.com"},
			want: false,
		},
		{
			name: "Domain name with unwanted suffix",
			args: args{domain: "suffix.domain.com/"},
			want: false,
		},
		{
			name: "Blank domain passed to function",
			args: args{domain: ""},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidDomain(tt.args.domain); got != tt.want {
				t.Errorf("IsValidDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasValidCustomDomainCR(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = customdomainv1alpha1.AddToScheme(scheme)

	type args struct {
		ctx          context.Context
		serverClient client.Client
		domain       string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "Valid CustomDomain found (1 CR)",
			args: args{
				domain: "apps.example.com",
				ctx:    context.TODO(),
				serverClient: fake.NewFakeClientWithScheme(scheme, &customdomainv1alpha1.CustomDomainList{
					Items: []customdomainv1alpha1.CustomDomain{
						{
							ObjectMeta: v1.ObjectMeta{
								Name: "goodDomain",
							},
							Spec: customdomainv1alpha1.CustomDomainSpec{
								Domain: "apps.example.com",
							},
							Status: customdomainv1alpha1.CustomDomainStatus{
								State: "Ready",
							},
						},
					},
				}),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Valid CustomDomain found (Multi CR)",
			args: args{
				domain: "apps.example.com",
				ctx:    context.TODO(),
				serverClient: fake.NewFakeClientWithScheme(scheme, &customdomainv1alpha1.CustomDomainList{
					Items: []customdomainv1alpha1.CustomDomain{
						{
							ObjectMeta: v1.ObjectMeta{
								Name: "badDomain",
							},
							Spec: customdomainv1alpha1.CustomDomainSpec{
								Domain: "bad.example.com",
							},
							Status: customdomainv1alpha1.CustomDomainStatus{
								State: "Failing",
							},
						},
						{
							ObjectMeta: v1.ObjectMeta{
								Name: "goodDomain",
							},
							Spec: customdomainv1alpha1.CustomDomainSpec{
								Domain: "apps.example.com",
							},
							Status: customdomainv1alpha1.CustomDomainStatus{
								State: "Ready",
							},
						},
					},
				}),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:    "Empty/invalid domain string passed to function",
			args:    args{domain: ""},
			want:    false,
			wantErr: true,
		},
		{
			name: "CustomDomain CR not in Ready state",
			args: args{
				domain: "apps.example.com",
				ctx:    context.TODO(),
				serverClient: fake.NewFakeClientWithScheme(scheme, &customdomainv1alpha1.CustomDomainList{
					Items: []customdomainv1alpha1.CustomDomain{
						{
							ObjectMeta: v1.ObjectMeta{
								Name: "goodDomain",
							},
							Spec: customdomainv1alpha1.CustomDomainSpec{
								Domain: "apps.example.com",
							},
							Status: customdomainv1alpha1.CustomDomainStatus{
								State: "Failing",
							},
						},
					},
				}),
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasValidCustomDomainCR(tt.args.ctx, tt.args.serverClient, tt.args.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasValidCustomDomainCR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HasValidCustomDomainCR() got = %v, want %v", got, tt.want)
			}
		})
	}
}
