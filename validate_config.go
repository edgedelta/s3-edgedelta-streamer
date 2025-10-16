package main

import (
	"fmt"
	"log"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/config"
)

func main() {
	// Load the config with all the new log formats
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	fmt.Printf("✅ Successfully loaded configuration with %d log formats:\n", len(cfg.Processing.LogFormats))

	for i, format := range cfg.Processing.LogFormats {
		fmt.Printf("%d. %s\n", i+1, format.Name)
		fmt.Printf("   Pattern: %s\n", format.FilenamePattern)
		fmt.Printf("   Regex: %s\n", format.TimestampRegex)
		fmt.Printf("   Format: %s\n", format.TimestampFormat)
		fmt.Printf("   Content-Type: %s\n", format.ContentType)
		if format.SkipHeaderLines > 0 {
			fmt.Printf("   Skip Headers: %d\n", format.SkipHeaderLines)
		}
		if format.FieldSeparator != "" {
			fmt.Printf("   Field Separator: %q\n", format.FieldSeparator)
		}
		fmt.Println()
	}

	fmt.Printf("Default format: %s\n", cfg.Processing.DefaultFormat)
	fmt.Println("✅ All log formats configured successfully!")
}
