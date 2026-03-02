// Package cloud implements the scraper.Mode interface for Cloud Consoles (AWS/GCP/Azure) extraction.
// It intercepts cloud provider API calls via session hijacking to capture structured
// resource, billing, IAM, and service data without DOM scraping.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// cloudProvider implements auth.Provider for cloud consoles.
type cloudProvider struct{}

func (p *cloudProvider) Name() string { return "cloud" }

func (p *cloudProvider) LoginURL() string { return "https://console.aws.amazon.com/" }

func (p *cloudProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("cloud: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)	if err != nil {
		return false, fmt.Errorf("cloud: detect auth: eval url: %w", err)
	}

	url := result.String()

	// Check if URL indicates authenticated state in any major cloud provider.
	if strings.Contains(url, "console.aws.amazon.com") ||
		strings.Contains(url, "console.cloud.google.com") ||
		strings.Contains(url, "portal.azure.com") {
		// Ensure we're not on a login/signin page.
		if !strings.Contains(url, "login") && !strings.Contains(url, "signin") {
			return true, nil
		}
	}

	return false, nil
}

func (p *cloudProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("cloud: capture session: nil page")
	}

	cookies, err := page.GetCookies()	if err != nil {
		return nil, fmt.Errorf("cloud: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)	if err != nil {
		return nil, fmt.Errorf("cloud: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)
	sessionStorage := make(map[string]string)

	// Extract AWS-specific tokens and data.
	if strings.Contains(currentURL, "console.aws.amazon.com") {
		// Try to extract aws-userInfo from localStorage or window object.
		awsUserResult, err := page.Eval(`() => {			try {
				const userInfo = localStorage.getItem('aws-userInfo');
				if (userInfo) return userInfo;
				if (window.__AWS_USERDATA) return JSON.stringify(window.__AWS_USERDATA);
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			userInfo := awsUserResult.String()
			if userInfo != "" {
				localStorage["aws-userInfo"] = userInfo
				tokens["aws-userinfo"] = userInfo
			}
		}

		// Extract AWS credentials or session tokens from sessionStorage.
		sessionTokenResult, err := page.Eval(`() => {
			try {
				for (let i = 0; i < sessionStorage.length; i++) {
					const key = sessionStorage.key(i);
					if (key && (key.includes('token') || key.includes('credential') || key.includes('session'))) {
						return JSON.stringify({ [key]: sessionStorage.getItem(key) });
					}
				}
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			sessionData := sessionTokenResult.String()
			if sessionData != "" {
				sessionStorage["aws-session"] = sessionData
			}
		}
	}

	// Extract GCP-specific tokens and data.
	if strings.Contains(currentURL, "console.cloud.google.com") {
		// Try to extract OSID (OAuth Session ID) and other GCP auth tokens.
		gcpOsidResult, err := page.Eval(`() => {
			try {
				const cookies = document.cookie.split(';');
				for (const cookie of cookies) {
					const [key, val] = cookie.split('=');
					if (key && key.trim() === 'OSID') return decodeURIComponent(val.trim());
				}
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			osid := gcpOsidResult.String()
			if osid != "" {
				tokens["gcp-osid"] = osid
			}
		}

		// Extract GCP project or workspace data from localStorage.
		gcpDataResult, err := page.Eval(`() => {
			try {
				const data = localStorage.getItem('goog_profile_data');
				if (data) return data;
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			gcpData := gcpDataResult.String()
			if gcpData != "" {
				localStorage["gcp_profile_data"] = gcpData
			}
		}
	}

	// Extract Azure-specific tokens and data.
	if strings.Contains(currentURL, "portal.azure.com") {
		// Try to extract Azure auth tokens and subscription info.
		azureTokenResult, err := page.Eval(`() => {
			try {
				const token = sessionStorage.getItem('msal.idtoken');
				if (token) return token;
				if (window.localStorage.getItem('msal.idtoken')) {
					return window.localStorage.getItem('msal.idtoken');
				}
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			azureToken := azureTokenResult.String()
			if azureToken != "" {
				tokens["azure-idtoken"] = azureToken
			}
		}

		// Extract subscription info from localStorage.
		azureSubResult, err := page.Eval(`() => {
			try {
				const sub = localStorage.getItem('subscription');
				if (sub) return sub;
				if (sessionStorage.getItem('azure-subscription')) {
					return sessionStorage.getItem('azure-subscription');
				}
			} catch(e) {}
			return '';
		}`)
		if err == nil {
			azureSub := azureSubResult.String()
			if azureSub != "" {
				sessionStorage["azure-subscription"] = azureSub
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:       "cloud",
		Version:        "1",
		Timestamp:      now,
		URL:            currentURL,
		Cookies:        cookies,
		Tokens:         tokens,
		LocalStorage:   localStorage,
		SessionStorage: sessionStorage,
		ExpiresAt:      now.Add(24 * time.Hour),
	}, nil
}

func (p *cloudProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("cloud: validate session: nil session")
	}

	// Check for presence of cloud provider-specific auth indicators.
	hasAWSAuth := false
	hasGCPAuth := false
	hasAzureAuth := false

	for k, v := range session.Tokens {
		if strings.Contains(k, "aws") || strings.Contains(v, "aws") {
			hasAWSAuth = true
		}

		if strings.Contains(k, "gcp") || strings.Contains(k, "osid") {
			hasGCPAuth = true
		}

		if strings.Contains(k, "azure") {
			hasAzureAuth = true
		}
	}

	// Also check in localStorage/SessionStorage for cloud provider markers.
	for k := range session.LocalStorage {
		if strings.Contains(k, "aws") {
			hasAWSAuth = true
		}

		if strings.Contains(k, "gcp") {
			hasGCPAuth = true
		}

		if strings.Contains(k, "azure") {
			hasAzureAuth = true
		}
	}

	for k := range session.SessionStorage {
		if strings.Contains(k, "aws") {
			hasAWSAuth = true
		}

		if strings.Contains(k, "azure") {
			hasAzureAuth = true
		}
	}

	if hasAWSAuth || hasGCPAuth || hasAzureAuth {
		return nil
	}

	return &scraper.AuthError{Reason: "no valid cloud provider auth tokens found in session"}
}

// CloudMode implements scraper.Mode for Cloud Consoles (AWS/GCP/Azure).
type CloudMode struct {
	provider cloudProvider
}

func (m *CloudMode) Name() string { return "cloud" }
func (m *CloudMode) Description() string {
	return "Scrape cloud consoles (AWS/GCP/Azure) resources, billing, IAM, and services"
}
func (m *CloudMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to the cloud console,
// and intercepts cloud provider API calls to extract structured data.
func (m *CloudMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	cloudSession, ok := session.(*auth.Session)
	if !ok || cloudSession == nil {
		return nil, fmt.Errorf("cloud: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, cloudSession); err != nil {
		return nil, fmt.Errorf("cloud: scrape: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	browser, err := scout.New(		scout.WithHeadless(opts.Headless),
		scout.WithStealth(),
	)
	if err != nil {
		return nil, fmt.Errorf("cloud: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(cloudSession.URL)	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("cloud: scrape: new page: %w", err)
	}

	if err := page.SetCookies(cloudSession.Cookies...); err != nil {		browser.Close()
		return nil, fmt.Errorf("cloud: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("cloud: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {		browser.Close()
		return nil, fmt.Errorf("cloud: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(		scout.WithHijackURLFilter("*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("cloud: scrape: create hijacker: %w", err)
	}

	results := make(chan scraper.Result, 256)
	targetSet := buildTargetSet(opts.Targets)

	go func() {
		defer close(results)
		defer hijacker.Stop()
		defer browser.Close()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count := 0

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-hijacker.Events():
				if !ok {
					return
				}

				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				items := parseHijackEvent(ev, targetSet)
				for _, item := range items {
					select {
					case <-ctx.Done():
						return
					case results <- item:
						count++
						if opts.Limit > 0 && count >= opts.Limit {
							return
						}

						if opts.Progress != nil {
							opts.Progress(scraper.Progress{
								Phase:   "scraping",
								Current: count,
								Total:   opts.Limit,
								Message: fmt.Sprintf("captured %d items", count),
							})
						}
					}
				}
			}
		}
	}()

	return results, nil
}

// buildTargetSet creates a lookup set from target service names or resource IDs.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from cloud API responses.
func parseHijackEvent(ev scout.HijackEvent, targetSet map[string]struct{}) []scraper.Result {
	if ev.Type != scout.HijackEventResponse || ev.Response == nil {
		return nil
	}

	url := ev.Response.URL

	body := ev.Response.Body
	if body == "" {
		return nil
	}

	// AWS API endpoints.
	switch {
	case strings.Contains(url, "console.aws.amazon.com/ec2"):
		return parseAWSEC2(body, targetSet)
	case strings.Contains(url, "console.aws.amazon.com/s3"):
		return parseAWSS3(body, targetSet)
	case strings.Contains(url, "console.aws.amazon.com/iam"):
		return parseAWSIAM(body, targetSet)
	case strings.Contains(url, "pricing.us-east-1.amazonaws.com"):
		return parseAWSPricing(body, targetSet)
	// GCP API endpoints.
	case strings.Contains(url, "console.cloud.google.com/_d/"):
		return parseGCPResources(body, targetSet)
	case strings.Contains(url, "cloudresourcemanager.googleapis.com"):
		return parseGCPProjects(body, targetSet)
	// Azure API endpoints.
	case strings.Contains(url, "management.azure.com"):
		return parseAzureResources(body, targetSet)
	case strings.Contains(url, "portal.azure.com/api"):
		return parseAzureAPI(body, targetSet)
	default:
		return nil
	}
}

// parseAWSEC2 extracts EC2 instances from AWS API responses.
func parseAWSEC2(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Check for reservations (from EC2 DescribeInstances).
	if reservations, ok := resp["Reservations"].([]any); ok {
		for _, r := range reservations {
			if res, ok := r.(map[string]any); ok {
				if instances, ok := res["Instances"].([]any); ok {
					for _, inst := range instances {
						if instMap, ok := inst.(map[string]any); ok {
							item := parseAWSInstance(instMap, targetSet)
							if item != nil {
								results = append(results, *item)
							}
						}
					}
				}
			}
		}
	}

	return results
}

func parseAWSInstance(inst map[string]any, targetSet map[string]struct{}) *scraper.Result {
	instanceID, _ := inst["InstanceId"].(string)
	if instanceID == "" {
		return nil
	}

	if targetSet != nil {
		if _, ok := targetSet[strings.ToLower(instanceID)]; !ok {
			if _, ok := targetSet["ec2"]; !ok {
				return nil
			}
		}
	}

	state, _ := inst["State"].(map[string]any)
	stateName, _ := state["Name"].(string)

	return &scraper.Result{
		Type:      scraper.ResultPost,
		Source:    "aws",
		ID:        instanceID,
		Timestamp: time.Now(),
		Content:   stateName,
		Metadata: map[string]any{
			"service": "ec2",
			"state":   stateName,
			"details": inst,
		},
		Raw: inst,
	}
}

// parseAWSS3 extracts S3 buckets from AWS API responses.
func parseAWSS3(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Check for Buckets list.
	if buckets, ok := resp["Buckets"].([]any); ok {
		for _, b := range buckets {
			if bucketMap, ok := b.(map[string]any); ok {
				bucketName, _ := bucketMap["Name"].(string)
				if bucketName == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(bucketName)]; !ok {
						if _, ok := targetSet["s3"]; !ok {
							continue
						}
					}
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultPost,
					Source:    "aws",
					ID:        bucketName,
					Timestamp: time.Now(),
					Content:   bucketName,
					Metadata: map[string]any{
						"service":        "s3",
						"bucket_name":    bucketName,
						"creation_date":  bucketMap["CreationDate"],
						"bucket_details": bucketMap,
					},
					Raw: bucketMap,
				})
			}
		}
	}

	return results
}

// parseAWSIAM extracts IAM users and roles from AWS API responses.
func parseAWSIAM(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Check for Users.
	if users, ok := resp["Users"].([]any); ok {
		for _, u := range users {
			if userMap, ok := u.(map[string]any); ok {
				userName, _ := userMap["UserName"].(string)
				if userName == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(userName)]; !ok {
						if _, ok := targetSet["iam"]; !ok {
							continue
						}
					}
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultUser,
					Source:    "aws",
					ID:        userName,
					Timestamp: time.Now(),
					Author:    userName,
					Metadata: map[string]any{
						"service":      "iam",
						"user_details": userMap,
						"arn":          userMap["Arn"],
						"create_date":  userMap["CreateDate"],
						"path":         userMap["Path"],
					},
					Raw: userMap,
				})
			}
		}
	}

	// Check for Roles.
	if roles, ok := resp["Roles"].([]any); ok {
		for _, r := range roles {
			if roleMap, ok := r.(map[string]any); ok {
				roleName, _ := roleMap["RoleName"].(string)
				if roleName == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(roleName)]; !ok {
						if _, ok := targetSet["iam"]; !ok {
							continue
						}
					}
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultChannel,
					Source:    "aws",
					ID:        roleName,
					Timestamp: time.Now(),
					Content:   roleName,
					Metadata: map[string]any{
						"service":      "iam",
						"role_type":    "role",
						"role_details": roleMap,
						"arn":          roleMap["Arn"],
						"create_date":  roleMap["CreateDate"],
						"path":         roleMap["Path"],
					},
					Raw: roleMap,
				})
			}
		}
	}

	return results
}

// parseAWSPricing extracts pricing information from AWS pricing API.
func parseAWSPricing(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, 1)

	// Extract pricing data as a message result.
	results = append(results, scraper.Result{
		Type:      scraper.ResultMessage,
		Source:    "aws",
		ID:        "pricing-snapshot",
		Timestamp: time.Now(),
		Content:   "AWS pricing data",
		Metadata: map[string]any{
			"service":      "pricing",
			"pricing_data": resp,
			"captured_at":  time.Now(),
		},
		Raw: resp,
	})

	return results
}

// parseGCPResources extracts GCP resources from Google Cloud API responses.
func parseGCPResources(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Generic resource extraction from API responses.
	if resources, ok := resp["resources"].([]any); ok {
		for _, r := range resources {
			if resMap, ok := r.(map[string]any); ok {
				resourceID, _ := resMap["name"].(string)
				if resourceID == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(resourceID)]; !ok {
						if _, ok := targetSet["gcp"]; !ok {
							continue
						}
					}
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultPost,
					Source:    "gcp",
					ID:        resourceID,
					Timestamp: time.Now(),
					Content:   resourceID,
					Metadata: map[string]any{
						"service":  "compute",
						"resource": resMap,
					},
					Raw: resMap,
				})
			}
		}
	}

	return results
}

// parseGCPProjects extracts GCP projects from Google Cloud Resource Manager API.
func parseGCPProjects(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Extract projects.
	if projects, ok := resp["projects"].([]any); ok {
		for _, p := range projects {
			if projMap, ok := p.(map[string]any); ok {
				projectID, _ := projMap["projectId"].(string)
				projectName, _ := projMap["name"].(string)

				if projectID == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(projectID)]; !ok {
						if _, ok := targetSet["gcp"]; !ok {
							continue
						}
					}
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultChannel,
					Source:    "gcp",
					ID:        projectID,
					Timestamp: time.Now(),
					Content:   projectName,
					Metadata: map[string]any{
						"service":     "resourcemanager",
						"project_id":  projectID,
						"project_num": projMap["projectNumber"],
						"status":      projMap["lifecycleState"],
						"project":     projMap,
					},
					Raw: projMap,
				})
			}
		}
	}

	return results
}

// parseAzureResources extracts Azure resources from Azure Management API.
func parseAzureResources(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Extract resources from value array.
	if resources, ok := resp["value"].([]any); ok {
		for _, r := range resources {
			if resMap, ok := r.(map[string]any); ok {
				resourceID, _ := resMap["id"].(string)

				resourceName, _ := resMap["name"].(string)
				if resourceID == "" || resourceName == "" {
					continue
				}

				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(resourceName)]; !ok {
						if _, ok := targetSet[strings.ToLower(resourceID)]; !ok {
							if _, ok := targetSet["azure"]; !ok {
								continue
							}
						}
					}
				}

				resType, _ := resMap["type"].(string)
				results = append(results, scraper.Result{
					Type:      scraper.ResultPost,
					Source:    "azure",
					ID:        resourceID,
					Timestamp: time.Now(),
					Content:   resourceName,
					Metadata: map[string]any{
						"service":     "management",
						"resource_id": resourceID,
						"type":        resType,
						"location":    resMap["location"],
						"tags":        resMap["tags"],
						"resource":    resMap,
					},
					Raw: resMap,
				})
			}
		}
	}

	return results
}

// parseAzureAPI extracts data from Azure portal API responses.
func parseAzureAPI(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Extract generic portal API data.
	if data, ok := resp["data"].(map[string]any); ok {
		// Try to extract subscriptions or other structured data.
		if subscriptions, ok := data["subscriptions"].([]any); ok {
			for _, sub := range subscriptions {
				if subMap, ok := sub.(map[string]any); ok {
					subID, _ := subMap["subscriptionId"].(string)
					subName, _ := subMap["displayName"].(string)

					if subID == "" {
						continue
					}

					if targetSet != nil {
						if _, ok := targetSet[strings.ToLower(subID)]; !ok {
							if _, ok := targetSet[strings.ToLower(subName)]; !ok {
								if _, ok := targetSet["azure"]; !ok {
									continue
								}
							}
						}
					}

					results = append(results, scraper.Result{
						Type:      scraper.ResultChannel,
						Source:    "azure",
						ID:        subID,
						Timestamp: time.Now(),
						Content:   subName,
						Metadata: map[string]any{
							"service":         "portal",
							"subscription_id": subID,
							"state":           subMap["state"],
							"subscription":    subMap,
						},
						Raw: subMap,
					})
				}
			}
		}
	}

	// Fallback: extract from top-level structure if available.
	if value, ok := resp["value"].([]any); ok {
		for _, v := range value {
			if vMap, ok := v.(map[string]any); ok {
				itemID, _ := vMap["id"].(string)
				itemName, _ := vMap["name"].(string)

				if itemID == "" {
					continue
				}

				results = append(results, scraper.Result{
					Type:      scraper.ResultPost,
					Source:    "azure",
					ID:        itemID,
					Timestamp: time.Now(),
					Content:   itemName,
					Metadata: map[string]any{
						"service": "portal",
						"item":    vMap,
					},
					Raw: vMap,
				})
			}
		}
	}

	return results
}

func init() {
	scraper.RegisterMode(&CloudMode{})
}
