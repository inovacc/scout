package engine

import (
	"fmt"
	"strings"
)

// GitHubRepo navigates to a GitHub repository page and extracts metadata.
func (b *Browser) GitHubRepo(owner, name string, opts ...GitHubOption) (*GitHubRepo, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, name)

	page, err := b.NewPage(repoURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate repo: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const repo = {};

		// Description
		const descEl = document.querySelector('[data-testid="about-description"], .f4.my-3, p.f4.my-3');
		repo.description = descEl ? descEl.textContent.trim() : '';

		// Stars
		const starsEl = document.querySelector('#repo-stars-counter-star, a[href$="/stargazers"] span, [id="repo-stars-counter-star"]');
		repo.stars = 0;
		if (starsEl) {
			const t = starsEl.textContent.trim().replace(/,/g, '');
			if (t.endsWith('k')) {
				repo.stars = Math.round(parseFloat(t) * 1000);
			} else {
				repo.stars = parseInt(t, 10) || 0;
			}
		}

		// Forks
		const forksEl = document.querySelector('#repo-network-counter, a[href$="/forks"] span, [id="repo-network-counter"]');
		repo.forks = 0;
		if (forksEl) {
			const t = forksEl.textContent.trim().replace(/,/g, '');
			if (t.endsWith('k')) {
				repo.forks = Math.round(parseFloat(t) * 1000);
			} else {
				repo.forks = parseInt(t, 10) || 0;
			}
		}

		// Language
		const langEl = document.querySelector('[data-ga-click*="language"], .BorderGrid-cell .h4, span[itemprop="programmingLanguage"]');
		repo.language = langEl ? langEl.textContent.trim() : '';

		// Topics
		repo.topics = [];
		document.querySelectorAll('a.topic-tag, a[data-octo-click="topic_click"]').forEach(el => {
			const t = el.textContent.trim();
			if (t) repo.topics.push(t);
		});

		// License
		const licenseEl = document.querySelector('a[data-analytics-event*="LICENSE"], a[href*="/blob/"] svg.octicon-law');
		if (licenseEl) {
			const parent = licenseEl.closest('a') || licenseEl;
			repo.license = parent.textContent.trim();
		} else {
			repo.license = '';
		}

		return repo;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval repo: %w", err)
	}

	repo := &GitHubRepo{
		Owner: owner,
		Name:  name,
	}

	if m, ok := result.Value.(map[string]any); ok {
		if v, ok := m["description"].(string); ok {
			repo.Description = v
		}

		if v, ok := m["stars"].(float64); ok {
			repo.Stars = int(v)
		}

		if v, ok := m["forks"].(float64); ok {
			repo.Forks = int(v)
		}

		if v, ok := m["language"].(string); ok {
			repo.Language = v
		}

		if v, ok := m["license"].(string); ok {
			repo.License = v
		}

		if topics, ok := m["topics"].([]any); ok {
			for _, t := range topics {
				if s, ok := t.(string); ok {
					repo.Topics = append(repo.Topics, s)
				}
			}
		}
	}

	// Optionally fetch README
	if cfg.includeBody {
		readmeURL := fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/README.md", owner, name)

		readmePage, readmeErr := b.NewPage(readmeURL)
		if readmeErr == nil {
			defer func() { _ = readmePage.Close() }()

			if readmePage.WaitLoad() == nil {
				md, mdErr := readmePage.Markdown(WithMainContentOnly())
				if mdErr == nil {
					repo.ReadmeMD = md
				}
			}
		}
	}

	return repo, nil
}

// GitHubUser navigates to a GitHub user profile and extracts metadata.
func (b *Browser) GitHubUser(username string) (*GitHubUser, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	profileURL := fmt.Sprintf("https://github.com/%s", username)

	page, err := b.NewPage(profileURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate user: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const user = {};

		// Display name
		const nameEl = document.querySelector('[itemprop="name"], .p-name');
		user.display_name = nameEl ? nameEl.textContent.trim() : '';

		// Username
		const loginEl = document.querySelector('[itemprop="additionalName"], .p-nickname');
		user.username = loginEl ? loginEl.textContent.trim() : '';

		// Bio
		const bioEl = document.querySelector('[data-bio-text], .p-note .user-profile-bio, [itemprop="description"]');
		user.bio = bioEl ? bioEl.textContent.trim() : '';

		// Location
		const locEl = document.querySelector('[itemprop="homeLocation"], .p-label');
		user.location = locEl ? locEl.textContent.trim() : '';

		// Counters from profile nav
		user.repos = 0;
		user.followers = 0;
		user.following = 0;

		// Try nav tabs
		document.querySelectorAll('a.UnderlineNav-item, nav a[data-tab-item]').forEach(a => {
			const text = a.textContent.trim().toLowerCase();
			const countEl = a.querySelector('.Counter, span.Counter');
			const count = countEl ? parseInt(countEl.textContent.trim().replace(/,/g, ''), 10) || 0 : 0;
			if (text.includes('repositories') || text.includes('repos')) {
				user.repos = count;
			}
		});

		// Followers/following from profile sidebar
		document.querySelectorAll('a[href*="tab=followers"], a[href*="tab=following"]').forEach(a => {
			const text = a.textContent.trim();
			const match = text.match(/([\d,.]+[km]?)/i);
			if (!match) return;
			let val = match[1].replace(/,/g, '');
			let num = 0;
			if (val.toLowerCase().endsWith('k')) {
				num = Math.round(parseFloat(val) * 1000);
			} else if (val.toLowerCase().endsWith('m')) {
				num = Math.round(parseFloat(val) * 1000000);
			} else {
				num = parseInt(val, 10) || 0;
			}
			if (a.href.includes('followers')) {
				user.followers = num;
			} else {
				user.following = num;
			}
		});

		return user;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval user: %w", err)
	}

	user := &GitHubUser{Username: username}

	if m, ok := result.Value.(map[string]any); ok {
		if v, ok := m["username"].(string); ok && v != "" {
			user.Username = v
		}

		if v, ok := m["display_name"].(string); ok {
			user.DisplayName = v
		}

		if v, ok := m["bio"].(string); ok {
			user.Bio = v
		}

		if v, ok := m["location"].(string); ok {
			user.Location = v
		}

		if v, ok := m["repos"].(float64); ok {
			user.Repos = int(v)
		}

		if v, ok := m["followers"].(float64); ok {
			user.Followers = int(v)
		}

		if v, ok := m["following"].(float64); ok {
			user.Following = int(v)
		}
	}

	return user, nil
}

// GitHubReleases navigates to the releases page of a GitHub repo and extracts release metadata.
func (b *Browser) GitHubReleases(owner, name string, opts ...GitHubOption) ([]GitHubRelease, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	cfg := githubDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	releasesURL := fmt.Sprintf("https://github.com/%s/%s/releases", owner, name)

	page, err := b.NewPage(releasesURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate releases: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(fmt.Sprintf(`() => {
		const items = [];
		const sections = document.querySelectorAll('[data-testid="release-card"], .release, section[aria-label]');
		const max = %d;

		for (let i = 0; i < sections.length && i < max; i++) {
			const sec = sections[i];
			const rel = {};

			// Tag
			const tagEl = sec.querySelector('a[href*="/releases/tag/"] span, a[href*="/releases/tag/"]');
			rel.tag = tagEl ? tagEl.textContent.trim() : '';

			// Name
			const nameEl = sec.querySelector('h2 a, [data-testid="release-title"] a');
			rel.name = nameEl ? nameEl.textContent.trim() : rel.tag;

			// Date
			const timeEl = sec.querySelector('relative-time, time');
			rel.date = timeEl ? (timeEl.getAttribute('datetime') || timeEl.textContent.trim()) : '';

			// Assets count
			const assetEls = sec.querySelectorAll('.Box a[href*="/releases/download/"]');
			rel.assets = assetEls.length;

			// Body (short, from the description)
			const bodyEl = sec.querySelector('.markdown-body');
			rel.body = bodyEl ? bodyEl.textContent.trim().substring(0, 500) : '';

			items.push(rel);
		}
		return items;
	}`, cfg.maxItems))
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval releases: %w", err)
	}

	var releases []GitHubRelease

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			rel := GitHubRelease{}
			if v, ok := m["tag"].(string); ok {
				rel.Tag = v
			}

			if v, ok := m["name"].(string); ok {
				rel.Name = v
			}

			if v, ok := m["body"].(string); ok {
				rel.Body = v
			}

			if v, ok := m["date"].(string); ok {
				rel.Date = v
			}

			if v, ok := m["assets"].(float64); ok {
				rel.Assets = int(v)
			}

			releases = append(releases, rel)
		}
	}

	return releases, nil
}

// GitHubTree navigates to a GitHub repo and extracts the file tree.
func (b *Browser) GitHubTree(owner, name, branch string) ([]string, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: github: browser is nil")
	}

	if branch == "" {
		branch = "HEAD"
	}

	treeURL := fmt.Sprintf("https://github.com/%s/%s/find/%s", owner, name, branch)

	page, err := b.NewPage(treeURL)
	if err != nil {
		return nil, fmt.Errorf("scout: github: navigate tree: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: github: wait load: %w", err)
	}

	result, err := page.Eval(`() => {
		const files = [];
		// The file finder page lists all files
		document.querySelectorAll('.tree-browser-result a, [data-testid="file-name-link"], .js-tree-browser-result-path').forEach(el => {
			const t = el.textContent.trim();
			if (t) files.push(t);
		});
		// Fallback: try the regular tree view
		if (files.length === 0) {
			document.querySelectorAll('.js-navigation-open[title], a.Link--primary[title]').forEach(el => {
				const t = el.getAttribute('title') || el.textContent.trim();
				if (t) files.push(t);
			});
		}
		return files;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: github: eval tree: %w", err)
	}

	var files []string

	if arr, ok := result.Value.([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					files = append(files, s)
				}
			}
		}
	}

	return files, nil
}
