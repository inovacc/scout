package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/knowledge"
	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(okfCmd)

	okfCmd.Flags().String("out", "./knowledge-bundle", "output directory for the OKF bundle")
	okfCmd.Flags().Int("depth", 0, "crawl depth (0 = seed only)")
	okfCmd.Flags().Int("max-pages", 20, "maximum pages to gather")
}

var okfCmd = &cobra.Command{
	Use:   "okf <url>",
	Short: "Gather a web page (or shallow crawl) and export it as an Open Knowledge Format bundle",
	Long: `Gather a web page or a same-domain shallow crawl and serialise the result
into an Open Knowledge Format (OKF) bundle directory.

Each visited page becomes a Markdown concept file. An index concept lists all
gathered pages with bundle-relative links.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		seedURL := args[0]
		outDir, _ := cmd.Flags().GetString("out")
		depth, _ := cmd.Flags().GetInt("depth")
		maxPages, _ := cmd.Flags().GetInt("max-pages")

		// Parse the seed URL to determine the allowed host for same-domain filtering.
		seedParsed, err := url.Parse(seedURL)
		if err != nil {
			return fmt.Errorf("scout: okf: invalid URL %q: %w", seedURL, err)
		}
		seedHost := seedParsed.Hostname()

		opts := baseOpts(cmd)
		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: okf: %w", err)
		}
		defer func() { _ = b.Close() }()

		// BFS queue: each entry carries the URL and its depth.
		type entry struct {
			rawURL string
			depth  int
		}

		queue := []entry{{rawURL: seedURL, depth: 0}}
		visited := make(map[string]bool)
		var pages []knowledge.PageInput

		for len(queue) > 0 && len(pages) < maxPages {
			cur := queue[0]
			queue = queue[1:]

			if visited[cur.rawURL] {
				continue
			}
			visited[cur.rawURL] = true

			in := tools.GatherInput{
				URL:        cur.rawURL,
				Markdown:   true,
				Links:      true,
				Frameworks: true,
				Meta:       true,
			}

			result, gErr := tools.Gather(context.Background(), b, in)
			if gErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: scout: okf: gather %q: %v\n", cur.rawURL, gErr)
				continue
			}

			// Extract framework names from the FrameworkInfo slice.
			fwNames := make([]string, 0, len(result.Frameworks))
			for _, fw := range result.Frameworks {
				fwNames = append(fwNames, fw.Name)
			}

			pages = append(pages, knowledge.PageInput{
				URL:        result.URL,
				Title:      result.Title,
				Markdown:   result.Markdown,
				Frameworks: fwNames,
				Links:      result.Links,
				Timestamp:  result.CollectedAt.UTC().Format(time.RFC3339),
			})

			// Enqueue same-host links if we can still descend.
			if cur.depth < depth {
				for _, link := range result.Links {
					if visited[link] {
						continue
					}
					parsed, parseErr := url.Parse(link)
					if parseErr != nil {
						continue
					}
					if parsed.Hostname() != seedHost {
						continue
					}
					queue = append(queue, entry{rawURL: link, depth: cur.depth + 1})
				}
			}
		}

		if len(pages) == 0 {
			return fmt.Errorf("scout: okf: no pages gathered from %q", seedURL)
		}

		bundle, err := knowledge.Build(seedURL, pages, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("scout: okf: build bundle: %w", err)
		}

		if err := bundle.Write(outDir); err != nil {
			return fmt.Errorf("scout: okf: write bundle: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "OKF bundle written to %s (%d concepts)\n", outDir, len(bundle.Concepts))
		return nil
	},
}
