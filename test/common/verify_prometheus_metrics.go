package common

import (
	"encoding/json"
	"fmt"
	integreatlyv1alpha1 "github.com/integr8ly/integreatly-operator/apis/v1alpha1"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"regexp"
)

func mangedApiTargets() map[string][]string {
	return map[string][]string{
		ObservabilityProductNamespace: {
			"/integreatly-3scale-admin-ui",
			"/integreatly-3scale-system-developer",
			"/integreatly-3scale-system-master",
			"/integreatly-grafana",
			"/integreatly-rhsso",
			"/integreatly-rhssouser",
			"/redhat-rhoam-cloud-resources-operator-cloud-resource-operator-metrics/0",
			"/redhat-rhoam-marin3r-ratelimit/0",
			"/redhat-rhoam-rhsso-keycloak-service-monitor/0",
			"/redhat-rhoam-rhsso-keycloak-service-monitor/1",
			"/redhat-rhoam-rhsso-operator-keycloak-operator-metrics/0",
			"/redhat-rhoam-rhsso-operator-keycloak-operator-metrics/1",
			"/redhat-rhoam-user-sso-keycloak-service-monitor/0",
			"/redhat-rhoam-user-sso-keycloak-service-monitor/1",
			"/redhat-rhoam-user-sso-operator-keycloak-operator-metrics/0",
			"/redhat-rhoam-user-sso-operator-keycloak-operator-metrics/1",
		},
	}
}

func mtMangedApiTargets() map[string][]string {
	return map[string][]string{
		ObservabilityProductNamespace: {
			"/integreatly-3scale-admin-ui",
			"/integreatly-3scale-system-developer",
			"/integreatly-3scale-system-master",
			"/integreatly-grafana",
			"/integreatly-rhsso",
			"/sandbox-rhoam-cloud-resources-operator-cloud-resource-operator-metrics/0",
			"/sandbox-rhoam-marin3r-ratelimit/0",
			"/sandbox-rhoam-rhsso-keycloak-service-monitor/0",
			"/sandbox-rhoam-rhsso-keycloak-service-monitor/1",
			"/sandbox-rhoam-rhsso-operator-keycloak-operator-metrics/0",
			"/sandbox-rhoam-rhsso-operator-keycloak-operator-metrics/1",
		},
	}
}

func TestMetricsScrappedByPrometheus(t TestingTB, ctx *TestingContext) {
	// get all active targets in prometheus
	output, err := execToPod("wget -qO - localhost:9090/api/v1/targets?state=active",
		"prometheus-prometheus-0",
		ObservabilityProductNamespace,
		"prometheus",
		ctx)
	if err != nil {
		t.Fatalf("failed to exec to prometheus pod: %s", err)
	}

	// get all found active targets from the prometheus api
	var promApiCallOutput prometheusAPIResponse
	err = json.Unmarshal([]byte(output), &promApiCallOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal json: %s", err)
	}

	var targetResult prometheusv1.TargetsResult
	err = json.Unmarshal(promApiCallOutput.Data, &targetResult)
	if err != nil {
		t.Fatalf("failed to unmarshal json: %s", err)
	}

	rhmi, err := GetRHMI(ctx.Client, true)
	if err != nil {
		t.Fatalf("error getting RHMI CR: %v", err)
	}

	// for every namespace
	for ns, targets := range getTargets(rhmi.Spec.Type) {
		// for every listed target in namespace
		for _, targetName := range targets {
			// check that metrics is being correctly scrapped by target
			correctlyScrapping := false
			for _, target := range targetResult.Active {
				if target.DiscoveredLabels["job"] == fmt.Sprintf("%s%s", ns, targetName) && target.Health == prometheusv1.HealthGood && target.ScrapeURL != "" {
					correctlyScrapping = true
					break
				}
			}

			if !correctlyScrapping {
				t.Errorf("Not correctly scrapping Prometheus target: %s%s", ns, targetName)
			}
		}
	}
}

func getTargets(installType string) map[string][]string {
	if integreatlyv1alpha1.IsRHOAMSingletenant(integreatlyv1alpha1.InstallationType(installType)) {
		return mangedApiTargets()
	} else if integreatlyv1alpha1.IsRHOAMMultitenant(integreatlyv1alpha1.InstallationType(installType)) {
		return mtMangedApiTargets()
	} else {
		// TODO - return list for managed install type
		return map[string][]string{}
	}
}

func TestRhoamVersionMetricExposed(t TestingTB, ctx *TestingContext) {
	const rhoamVersionKey = "rhoam_version"
	// Get the rhoam_version metric from prometheus
	promQueryRes, err := queryPrometheus(rhoamVersionKey, "prometheus-prometheus-0", ctx)
	if err != nil {
		t.Fatalf("Failed to query prometheus: %w", err)
	}
	if len(promQueryRes) == 0 {
		t.Fatalf("No results for metric %s ", rhoamVersionKey)
	}
	rhoamVersionValue := promQueryRes[0].Metric["version"].(string)
	// Semver regex (https://regexr.com/39s32)
	re := regexp.MustCompile(`^((([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?)(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?)$`)
	if !re.MatchString(rhoamVersionValue) {
		t.Fatalf("Failed to validate RHOAM version format. Expected semantic version, got %s", rhoamVersionValue)
	}
}