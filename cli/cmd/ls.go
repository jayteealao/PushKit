package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/progress"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List uploaded files",
	RunE:    runLs,
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
