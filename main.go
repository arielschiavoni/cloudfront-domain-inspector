package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	rgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgtTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
)

func main() {
	// Define a command-line flag for the stack name.
	stackName := flag.String("stack-name", "", "The stack name to filter resources")
	flag.Parse()

	if *stackName == "" {
		log.Fatalf("You must specify a stack name using the -stack-name flag.")
	}

	ctx := context.Background()

	// Load the AWS configuration using the default options.
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Create clients for the Resource Groups Tagging API and CloudFront.
	tagClient := rgt.NewFromConfig(cfg)
	cfClient := cf.NewFromConfig(cfg)

	// Log and send the GetResources request to fetch resources with the specified tag,
	// filtering for CloudFront distributions.
	log.Printf("Sending GetResources request to fetch resources with tag stack-name=%s", *stackName)
	resourcesOutput, err := tagClient.GetResources(ctx, &rgt.GetResourcesInput{
		TagFilters: []rgtTypes.TagFilter{
			{
				Key:    aws.String("stack-name"),
				Values: []string{*stackName},
			},
		},
		ResourceTypeFilters: []string{"cloudfront:distribution"},
	})
	if err != nil {
		log.Fatalf("failed to get resources by tag: %v", err)
	}

	if len(resourcesOutput.ResourceTagMappingList) == 0 {
		fmt.Println("No resources found with the specified tag.")
		return
	}

	log.Printf("Found %d distributions for stack-name=%s", len(resourcesOutput.ResourceTagMappingList), *stackName)

	totalDomains := 0

	// Process each resource that is of type CloudFront distribution.
	for _, resourceMapping := range resourcesOutput.ResourceTagMappingList {
		if resourceMapping.ResourceARN == nil {
			continue
		}

		arn := *resourceMapping.ResourceARN
		// Expecting ARN format: arn:aws:cloudfront::<account-id>:distribution/<distribution-id>
		parts := strings.Split(arn, "/")
		if len(parts) < 2 {
			log.Printf("unexpected ARN format: %s", arn)
			continue
		}
		distributionID := parts[len(parts)-1]

		log.Printf("Processing CloudFront distribution: %s (ARN: %s)", distributionID, arn)

		distConfigOutput, err := cfClient.GetDistributionConfig(ctx, &cf.GetDistributionConfigInput{
			Id: aws.String(distributionID),
		})
		if err != nil {
			log.Printf("failed to get configuration for distribution %s: %v", distributionID, err)
			continue
		}

		// Print the distribution and its SAN domains (if any).
		if distConfigOutput.DistributionConfig.Aliases != nil &&
			len(distConfigOutput.DistributionConfig.Aliases.Items) > 0 {
			fmt.Printf("Distribution ID: %s\n", distributionID)
			fmt.Println("SAN domains:")
			for _, alias := range distConfigOutput.DistributionConfig.Aliases.Items {
				fmt.Printf(" - %s\n", alias)
				totalDomains++
			}
			fmt.Println()
		} else {
			fmt.Printf("Distribution ID: %s has no SAN domains.\n\n", distributionID)
		}
	}

	// Print the total amount of SAN domains found.
	fmt.Printf("Total SAN domains found: %d\n", totalDomains)
}
