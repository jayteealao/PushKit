package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	apiclient "github.com/pushkit/cli/internal/client"
	"github.com/pushkit/cli/internal/progress"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List uploaded files",
	Long: `List files stored in PushKit.

Results are paginated. By default, the most recent 20 files are shown.
Use --all to fetch every page, or --limit to control page size.

Filtering and sorting:
  -q          Search by filename substring
  --sort      Sort field: created_at (default), original_filename, size_bytes
  --order     Sort direction: desc (default), asc

JSON output (--json):
  On success, prints a JSON object to stdout:
    {"items":[...],"nextCursor":"..."}
  Each item has: id, originalFilename, contentType, sizeBytes, createdAt, status.
  nextCursor is null when there are no more pages.`,
	Example: `  # List recent files
  pushkit ls

  # Search for PDFs, sorted by size
  pushkit ls -q .pdf --sort size_bytes --order desc

  # Get all files as JSON (for scripts/agents)
  pushkit ls --all --json`,
	RunE: runLs,
}

var (
	lsSearch string
	lsSort   string
	lsOrder  string
	lsLimit  int
	lsAll    bool
)

func init() {
	lsCmd.Flags().StringVarP(&lsSearch, "q", "q", "", "Search by filename")
	lsCmd.Flags().StringVar(&lsSort, "sort", "created_at", "Sort by: created_at, original_filename, size_bytes")
	lsCmd.Flags().StringVar(&lsOrder, "order", "desc", "Order: asc, desc")
	lsCmd.Flags().IntVar(&lsLimit, "limit", 20, "Results per page")
	lsCmd.Flags().BoolVar(&lsAll, "all", false, "Fetch all pages")
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	if flagJSON {
		return runLsJSON(ctx, c)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "ID\tFILENAME\tSIZE\tTYPE\tDATE\n")

	cursor := ""
	total := 0

	for {
		resp, err := c.ListFiles(ctx, cursor, lsLimit, lsSearch, lsSort, lsOrder)
		if err != nil {
			return fmt.Errorf("list files: %w", err)
		}

		for _, f := range resp.Items {
			sizeStr := "-"
			if f.SizeBytes != nil {
				sizeStr = progress.FormatBytes(*f.SizeBytes)
			}
			dateStr := f.CreatedAt
			if len(dateStr) >= 10 {
				dateStr = dateStr[:10]
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				f.ID, f.OriginalFilename, sizeStr, f.ContentType, dateStr)
		}
		total += len(resp.Items)

		if !lsAll || resp.NextCursor == nil {
			if resp.NextCursor != nil {
				fmt.Fprintf(os.Stderr, "\n(%d files shown, more available — use --all to see all)\n", total)
			}
			break
		}
		cursor = *resp.NextCursor
	}

	tw.Flush()

	if total == 0 {
		fmt.Println("No files found.")
	}
	return nil
}

// runLsJSON collects all requested pages and outputs a single JSON object.
func runLsJSON(ctx context.Context, c *apiclient.Client) error {
	var allItems []apiclient.FileResponse
	cursor := ""
	var lastCursor *string

	for {
		resp, err := c.ListFiles(ctx, cursor, lsLimit, lsSearch, lsSort, lsOrder)
		if err != nil {
			return fmt.Errorf("list files: %w", err)
		}

		allItems = append(allItems, resp.Items...)

		if !lsAll || resp.NextCursor == nil {
			lastCursor = resp.NextCursor
			break
		}
		cursor = *resp.NextCursor
	}

	if allItems == nil {
		allItems = []apiclient.FileResponse{}
	}

	return outputJSON(apiclient.FileListResponse{
		Items:      allItems,
		NextCursor: lastCursor,
	})
}
