package resources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"go.uber.org/zap"
)

func GetCFNStacksFromRegex(logger *zap.Logger, stackRegexes []string, region string) []string {
    stackIds := []string{}
    stackIdsFiltered := []string{}

	// Using the SDK's default configuration, loading additional config
    // and credentials values from the environment variables, shared
    // credentials, and shared configuration files
    cfg, err := config.LoadDefaultConfig(context.TODO(),
   		config.WithRegion(region),
   	)
    if err != nil {
        logger.Error(fmt.Sprintf("unable to load SDK config, %v", err))
    }

    // Using the Config value, create the cloudformation client
    svc := cloudformation.NewFromConfig(cfg)

    // Build the request with its input parameters
    resp, err := svc.ListStacks(context.TODO(), &cloudformation.ListStacksInput{
    })
    if err != nil {
        logger.Error(fmt.Sprintf("failed to get stacks, %v", err))
    }

    // Filter the CFN stacks based on provided regex
    for i, stackSummary := range resp.StackSummaries {
        
        stackIds = append(stackIds, *stackSummary.StackName)

        for _, stackRegex := range stackRegexes {
            match, err := regexp.MatchString(stackRegex, stackIds[i])
            if err != nil {
                fmt.Println("Error ", err)
            } else {
                if match {
                    logger.Debug(fmt.Sprintf("Regex match: %v\n", stackIdsFiltered))
                    stackIdsFiltered = append(stackIdsFiltered, stackIds[i])
                    break
                }
            }
        }
    }

    return stackIdsFiltered
}


func GetCFNStackChildren(logger *zap.Logger, stackName string, region string) map[string][]string {
    resourcesByType := make(map[string][]string)

	// Using the SDK's default configuration, loading additional config
    // and credentials values from the environment variables, shared
    // credentials, and shared configuration files
    cfg, err := config.LoadDefaultConfig(context.TODO(),
   		config.WithRegion(region),
   	)
    if err != nil {
        logger.Error(fmt.Sprintf("unable to load SDK config, %v", err))
    }

    // Using the Config value, create the cloudformation client
    svc := cloudformation.NewFromConfig(cfg)

    // Build the request with its input parameters
    resp, err := svc.ListStackResources(context.TODO(), &cloudformation.ListStackResourcesInput{
        StackName: &stackName,
    })
    if err != nil {
        logger.Error(fmt.Sprintf("failed to get stack resources, %v", err))
    }
    if len(resp.StackResourceSummaries) != 0 {
        for _, stackResourceSummary := range resp.StackResourceSummaries {
            if stackResourceSummary.PhysicalResourceId != nil {
                resourcesByType[*stackResourceSummary.ResourceType] = append(resourcesByType[*stackResourceSummary.ResourceType], *stackResourceSummary.PhysicalResourceId)
            } else {
                logger.Warn(fmt.Sprintf("A child of stack %v has no physical resource ID, and so won't be added to the preservation list.", &stackName))
                fmt.Printf("A child of stack %v has no physical resource ID, and so won't be added to the preservation list.%v\n", &stackName, stackResourceSummary)
            }
        }
    }

    return resourcesByType
}