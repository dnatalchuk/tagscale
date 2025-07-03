package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

var (
	applyProfile string
	applyTags    []string
	dryRun       bool
)

func init() {
	var applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply missing tags to AWS resources",
		Run:   runApply,
	}

	applyCmd.Flags().StringVar(&applyProfile, "profile", "default", "AWS CLI profile name")
	applyCmd.Flags().StringSliceVar(&applyTags, "tags", []string{}, "Tags to apply in key=value format (e.g. team=infra)")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", true, "Run without making changes")

	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) {
	fmt.Println("Starting TagScale apply process...")

	if len(applyTags) == 0 {
		log.Fatal("No tags provided. Use --tags key=value.")
	}

	// Parse tag strings into AWS types
	tags := []types.Tag{}
	for _, t := range applyTags {
		parts := strings.SplitN(t, "=", 2)
		if len(parts) != 2 {
			log.Fatalf("Invalid tag format: %s. Must be key=value.", t)
		}
		tags = append(tags, types.Tag{
			Key:   aws.String(parts[0]),
			Value: aws.String(parts[1]),
		})
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(applyProfile),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)

	// Example: find all EC2 instances in "running" state
	input := &ec2.DescribeInstancesInput{}
	result, err := ec2Client.DescribeInstances(context.TODO(), input)
	if err != nil {
		log.Fatalf("Failed to describe instances: %v", err)
	}

	var instanceIDs []string

	// Collect instance IDs to tag
	for _, res := range result.Reservations {
		for _, inst := range res.Instances {
			instanceIDs = append(instanceIDs, *inst.InstanceId)
		}
	}

	if len(instanceIDs) == 0 {
		fmt.Println("No EC2 instances found to tag.")
		return
	}

	fmt.Printf("Found %d instances to tag:\n", len(instanceIDs))
	for _, id := range instanceIDs {
		fmt.Println(" -", id)
	}

	if dryRun {
		fmt.Println("Dry-run enabled. No tags will be applied.")
		for _, id := range instanceIDs {
			fmt.Printf("Would tag instance %s with:\n", id)
			for _, t := range tags {
				fmt.Printf("   %s = %s\n", *t.Key, *t.Value)
			}
		}
	} else {
		// Actually apply tags
		for _, id := range instanceIDs {
			_, err := ec2Client.CreateTags(context.TODO(), &ec2.CreateTagsInput{
				Resources: []string{id},
				Tags:      tags,
			})
			if err != nil {
				log.Printf("Failed to tag instance %s: %v", id, err)
			} else {
				fmt.Printf("Tagged instance %s successfully.\n", id)
			}
		}
	}
}
