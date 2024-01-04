package main

import (
	"awsnukeshield/helpers"
	"awsnukeshield/resources"
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AWS regions to search for resources to preserve. By default, use all EU and US regions which are enabled by default in AWS accounts
var aws_regions = []string {"eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "eu-central-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2"}

// Overwrite the contents of the configFile with the contents of lines
func writeLinesToFile(logger *zap.Logger, configFile string, lines []string) {
    fmt.Println("\n\nWriting to the config file...")

    // Open the file for writing (truncating the existing content)
    file, err := os.Create(configFile)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Write the provided lines back to the file
    writer := bufio.NewWriter(file)
    for _, line := range lines {
        _, err := writer.WriteString(line + "\n")
        if err != nil {
            panic(err)
        }
    }
    writer.Flush()
}

// Write the additional resource filters to the lines array, in preparation for being written back to the config file
// To add more, add them into this function, copying one of the existing filters as an example
// This should only be used for writing resources for preservation which will not be captured by provided stack regexes or tags
// If unwanted, simply don't call the function from main
func addAdditionalFilters(logger *zap.Logger, lines []string) []string {
    additionalFilters := make(map[string][]string)
    additionalFilters["IAMRole"] = append(additionalFilters["IAMRole"], `        - "AWSCloudFormationStackSetExecutionRole"`)
    additionalFilters["IAMRole"] = append(additionalFilters["IAMRole"], `        - property: Name
          type: contains
          value: "stacksets-exec"`)
    additionalFilters["IAMRolePolicy"] = append(additionalFilters["IAMRolePolicy"], `        - property: role:RoleName
          value: "AWSCloudFormationStackSetExecutionRole"`)
    additionalFilters["IAMRolePolicyAttachment"] = append(additionalFilters["IAMRolePolicyAttachment"], `        - property: RoleName
          type: contains
          value: "stacksets-exec"`)
    additionalFilters["IAMSAMLProvider"] = append(additionalFilters["IAMSAMLProvider"], `        - type: contains
          value: "DO_NOT_DELETE"`)
    additionalFilters["SNSSubscription"] = append(additionalFilters["SNSSubscription"], `        - type: contains
          value: "aws-controltower"`)
    additionalFilters["CloudWatchEventsRule"] = append(additionalFilters["CloudWatchEventsRule"], `        - type: contains
          value: "aws-controltower"`)
    additionalFilters["CloudWatchEventsTarget"] = append(additionalFilters["CloudWatchEventsTarget"], `        - type: contains
          value: "aws-controltower"`)

    for key, filters := range additionalFilters {
        // Add the resources for preservation to the correct resource section of the file, under filters
        index_of_resource_block := helpers.FindItem(lines, fmt.Sprintf("%s:", key))
        indexToInsert := index_of_resource_block + 1
        lines = append(lines[:indexToInsert], append(filters, lines[indexToInsert:]...)...)
        logger.Debug(fmt.Sprintf("New contents of the config file: %v", lines))
    }

    return lines
}


// Write the contents of filter_contents to the lines array, in preparation for being written back to the config file
// The content is inserted at the beginning of the filter section of the file, with resource IDs as child elements of resource type keys
func generateResourceConfigSection(logger *zap.Logger, lines []string, filter_contents map[string][]string) []string {

    fmt.Println("\n\nNORMALISING RESOURCE TYPES:")
    fmt.Println()
    fmt.Println("The CFN API uses different resource type names to those accepted by aws-nuke. For each resource type which we want to preserve, we now attempt to map it to an aws-nuke resource type.")
    fmt.Println("If there is no direct match, you will be presented with options to choose between.")
    fmt.Println()

    // Get the aws-nuke resource types
    awsNukeResourceTypesTemp, err := exec.Command("bash", "-c", "aws-nuke resource-types").Output()
    if err != nil {
        panic(err)
    }

    resourceTypesString := string(awsNukeResourceTypesTemp)
    awsNukeResourceTypes := strings.Split(resourceTypesString, "\n")
    unmatchedResources := make(map[string][]string)

    // Certain aws-nuke resources can only be filtered in the config file through use of a specific property. customProperties maps resource type to the property (and optionally value).
    // When we add the resource to the config file, we use the custom property from the map, if present, else just use the default mechanism.
    // If the customProperties key has two elements, the first is the property, and the second the value
    customProperties := make(map[string][]string)
    customProperties["SNSTopic"] = append(customProperties["SNSTopic"], "TopicARN")
    customProperties["CloudFormationStack"] = append(customProperties["CloudFormationStack"], "Name")
    
    for key, resources := range filter_contents {
        // Map the resource type to one supported by aws-nuke
        
        var allPossibleMatches []string
        var chosenAwsNukeKey string

        cleanedKey := strings.Replace(key, "AWS::", "", -1)
        awsNukeKeySplit := strings.Split(cleanedKey, "::")
        for i:=0 ; i<len(awsNukeKeySplit) ; i++ {
            stringToSearch := ""

            for j := i; j < len(awsNukeKeySplit); j++ {
                stringToSearch += awsNukeKeySplit[j]
            }
            
            logger.Debug(fmt.Sprintf("\nSearching aws-nuke resource types for %s\n", stringToSearch))

            possibleMatches := helpers.FindItemAll(awsNukeResourceTypes, stringToSearch)

            logger.Debug(fmt.Sprintf("\nPossible matches: %v\n", possibleMatches))
            findIndex := helpers.FindItemExact(possibleMatches, stringToSearch)
            if findIndex != -1 {
                chosenAwsNukeKey = possibleMatches[findIndex]
                fmt.Printf("Mapped %s to type %s\n", key, chosenAwsNukeKey)
                break 
            } else {
                allPossibleMatches = append(allPossibleMatches, possibleMatches...)
            }         
        }

        if chosenAwsNukeKey == "" && len(allPossibleMatches) != 0 {
            // There was no exact mapping for this resource type, so let the user choose from a list of partial matches

            var resourceTypeChoice string

            allPossibleMatches = helpers.RemoveDuplicates[string](allPossibleMatches)

            fmt.Printf("There was no exact mapping found for %s in the list of aws-nuke resource types. There were some partial matches.\n", key)
            fmt.Println("Should you see the correct type in the list below, please type the corresponding number to use it for this mapping. If none, type -1. If -1, the resource will be omitted from the config file:")

            for i, possibleMatch := range allPossibleMatches {
                fmt.Printf("[%d] %s\n", i+1, possibleMatch)
            }

            fmt.Scanln(&resourceTypeChoice)
            choiceInt, err := strconv.Atoi(resourceTypeChoice)
            if err != nil {
                fmt.Println("Invalid option. All resources of this type will therefore be omitted from the config file.")
            }

            if choiceInt != -1 {
                if choiceInt > len(allPossibleMatches) || choiceInt<0{
                    fmt.Println("Invalid option. All resources of this type will therefore be omitted from the config file.")
                } else{
                    chosenAwsNukeKey = allPossibleMatches[choiceInt-1]
                }
            } else{
                fmt.Println("All mapping options refused. All resources of this type will therefore be omitted from the config file.")
            }
        } else if chosenAwsNukeKey == "" && len(allPossibleMatches) == 0{
            fmt.Printf("Failed to find any suitable aws-nuke resource type to map to %s. All resources of this type will therefore be omitted from the config file.\n", key)
        }

        if chosenAwsNukeKey != "" {
            var filterContents []string
            // Format and add the resource names of this type to the filterContents, ready to be written to the config file
    
            customProperty, customPropertyExists := customProperties[chosenAwsNukeKey]

            for _, resource := range resources {
                if customPropertyExists {
                    if len(customProperty) ==1 {
                        filterContents = append(filterContents, fmt.Sprintf("        - property: \"%s\"", customProperty[0]))
                        filterContents = append(filterContents, fmt.Sprintf("          value: \"%s\"", resource))
                    } else {
                        filterContents = append(filterContents, fmt.Sprintf("        - property: \"%s\"", customProperty[0]))
                        filterContents = append(filterContents, fmt.Sprintf("          value: \"%s\"", customProperty[1]))
                    }
                    
                } else {
                    filterContents = append(filterContents, fmt.Sprintf("        - \"%s\"", resource))
                }
            }

            // Add the resources for preservation to the correct resource section of the file, under filters
            index_of_resource_block := helpers.FindItem(lines, fmt.Sprintf("%s:", chosenAwsNukeKey))
            indexToInsert := index_of_resource_block + 1
            lines = append(lines[:indexToInsert], append(filterContents, lines[indexToInsert:]...)...)
            logger.Debug(fmt.Sprintf("New contents of the config file: %v", lines))

            if chosenAwsNukeKey == "IAMRole" {
                // The IAMRole resources are a special case, whereby if we do nothing, their associated IAMRolePolicy and IAMRolePolicyAttachment resources won't also be preserved.
                // We must therefore add them to the file here.
                var iamRolePolicyContents []string
                var iamRolePolicyAttachmentContents []string
                for _, resource := range resources {
                    iamRolePolicyContents = append(iamRolePolicyContents, "        - property: role:RoleName")
                    iamRolePolicyContents = append(iamRolePolicyContents, fmt.Sprintf("          value: %s", resource))

                    iamRolePolicyAttachmentContents = append(iamRolePolicyAttachmentContents, "        - property: RoleName")
                    iamRolePolicyAttachmentContents = append(iamRolePolicyAttachmentContents, fmt.Sprintf("          value: %s", resource))
                }

                index_of_iamrolepolicy_block := helpers.FindItem(lines, fmt.Sprintf("%s:", "IAMRolePolicy"))
                indexToInsert = index_of_iamrolepolicy_block + 1
                lines = append(lines[:indexToInsert], append(iamRolePolicyContents, lines[indexToInsert:]...)...)

                index_of_iamrolepolicyattachment_block := helpers.FindItem(lines, fmt.Sprintf("%s:", "IAMRolePolicyAttachment"))
                indexToInsert = index_of_iamrolepolicyattachment_block + 1
                lines = append(lines[:indexToInsert], append(iamRolePolicyAttachmentContents, lines[indexToInsert:]...)...)

                logger.Debug(fmt.Sprintf("New contents of the config file: %v", lines))
            }
        
            
        } else {
            // An aws-nuke resource type wasn't matched. Add this to a slice to deal with later
            unmatchedResources[key] = append(unmatchedResources[key], resources...)
        }
        
    }

    fmt.Println("\n\nResource type normalisation complete.")
    if len(unmatchedResources) != 0 {
        fmt.Println("The following resources (grouped by type) could not be mapped, they have therefore NOT been added to the config file:")
        for unmatchedResourceType, unmatchedResourceName := range(unmatchedResources) {
            fmt.Printf("- %s: %s\n", unmatchedResourceType, unmatchedResourceName)
        }
        fmt.Println("\nIt may be that the resource types are not supported by aws-nuke, and therefore resources of this type will not be deleted.")
        fmt.Println("Run aws-nuke resource-types to see the full list of supported types.")
    }
    
    fmt.Println("\nPlease ensure that you review the generated config file, and review the resources aws-nuke marks for deletion before confirming the deletion!")    
    return lines
}

func GenerateResourceTypeConfigSection(logger *zap.Logger, lines []string, resourceTypesToFilter []string) []string {
	var filterContents []string
	
	for _, resourceType := range resourceTypesToFilter {
		if len(resourceType) != 0 {
			filterContents = append(filterContents, fmt.Sprintf("  - %s", resourceType))
		}
	}

    // Append the new section to the file
    indexOfResourceTypesBlock := helpers.FindItem(lines, "excludes:")
    if indexOfResourceTypesBlock != -1 {
        indexToInsert := indexOfResourceTypesBlock + 1
        lines = append(lines[:indexToInsert], append(filterContents, lines[indexToInsert:]...)...)

    } else {
        filterContents = append([]string{"  excludes:"}, filterContents...)
        filterContents = append([]string{"resource-types:"}, filterContents...)
        lines = append(lines, filterContents...)
    }
    logger.Debug(fmt.Sprintf("Added resource-types section for preservation of specified resources: %v", lines))

    return lines
	
}

func GenerateRolePolicyPreservationSection(logger *zap.Logger, lines []string, resourceTypesToFilter []string) []string {
	var filterContents []string
	
	for _, resourceType := range resourceTypesToFilter {
		if len(resourceType) != 0 {
			filterContents = append(filterContents, fmt.Sprintf("  - %s", resourceType))
		}
	}

    // Append the new section to the file
    indexOfResourceTypesBlock := helpers.FindItem(lines, "resource-types:")
    if indexOfResourceTypesBlock != -1 {
        indexToInsert := indexOfResourceTypesBlock + 1
        lines = append(lines[:indexToInsert], append(filterContents, lines[indexToInsert:]...)...)
    } else {
        filterContents = append([]string{"  excludes:"}, filterContents...)
        filterContents = append([]string{"resource-types:"}, filterContents...)
        lines = append(lines, filterContents...)
    }
    logger.Debug(fmt.Sprintf("Added resource-types section for preservation of specified resources: %v", lines))

    return lines
	
}

func main() {
    var stacksRegexes helpers.StringListFlag
    var resourceTags helpers.StringListFlag
    var resourceTypesToFilter helpers.StringListFlag
    var noDryRun bool
    var stackIdsFiltered []string
    var configFile string
    var generatedConfigFile string
    resourcesToPreserveByType := make(map[string][]string)

    // Configure logging options
    config := zap.Config{
        Level:             zap.NewAtomicLevelAt(zapcore.ErrorLevel), // e.g., InfoLevel, DebugLevel, ErrorLevel
        Encoding:          "json",
        OutputPaths:       []string{"stdout"},
        ErrorOutputPaths:  []string{"stderr"},     
        EncoderConfig:     zap.NewProductionEncoderConfig(),
        InitialFields:     map[string]interface{}{"app": "aws-nuke-wrapper"},
        DisableStacktrace: true,
    }

    // Create a logger instance
    logger, err := config.Build()
    if err != nil {
        panic(err)
    }
    defer logger.Sync()

    // Get CLI args
    flag.StringVar(&configFile, "config", "example-nuke-config.yml", "Base config file to use")
    flag.Var(&stacksRegexes, "regexes", "List of regexes to use to match cfn stack IDs")
    flag.Var(&resourceTags, "tags", "List of tags in key:value format. All resources with these tags will be preserved")
    flag.Var(&resourceTypesToFilter, "preserve-resource-types", "List of resource-types, in format accepted by aws-nuke. No resources of these types will be nuked")
    flag.BoolVar(&noDryRun, "no-dry-run", false, "Runs aws-nuke with the no-dry-run flag. WARNING this will cause aws-nuke to actually delete the resources in your account")
    flag.Parse()

    generatedConfigFile = fmt.Sprintf("%v-shield-generated", configFile)

    fmt.Println("PROVIDED REGEXES:")
    for _, regex := range stacksRegexes {
        fmt.Printf("\n%v", regex)
    }
    logger.Debug(fmt.Sprintf("Provided CFN stack regexes: %v", stacksRegexes))

    fmt.Println("\n\nREGIONS TO SEARCH:")
    fmt.Println()
    fmt.Printf("%s\n", aws_regions)

    fmt.Println("\n\nFinding resources...")
    for _, region := range aws_regions {
        // Get all the CFN stacks which match the provided regexes
        var regionalStackIdsFiltered = resources.GetCFNStacksFromRegex(logger, stacksRegexes, region)
        stackIdsFiltered = append(stackIdsFiltered, regionalStackIdsFiltered...) 

        // Get the child resources for each CFN stack, and group them by resource type (e.g. IAMRole)
        for _, stackId := range regionalStackIdsFiltered {
            for resourceType, resources := range resources.GetCFNStackChildren(logger, stackId, region){
                resourcesToPreserveByType[resourceType] = append(resourcesToPreserveByType[resourceType], resources...)
            }
            // Add the stacks themselves to the resources to preserve 
            resourcesToPreserveByType["CloudFormationStack"] = append(resourcesToPreserveByType["CloudFormationStack"], stackId)
        }
    }

    fmt.Println("\n\nSTACKS MATCHING REGEXES:")
    for _, stackId := range stackIdsFiltered {
        fmt.Printf("\n%s", stackId)
    }

    logger.Debug(fmt.Sprintf("Stacks to preserve the child resources of: %v\n", stackIdsFiltered))
    fmt.Println("\n\nRESOURCES TO PRESERVE:")
    for resourceType, resources := range resourcesToPreserveByType {
        fmt.Printf("\n%s: %v", resourceType, resources)
    }

    logger.Debug(fmt.Sprintf("Child resources to preserve: %v\n", resourcesToPreserveByType))

    // Read the provided config file

    // Open the file for reading
    file, err := os.Open(configFile)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Read the current contents of the file into a slice of strings, such that content can be inserted and they can then be written back to the file
    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }

    // Build up the new contents of the config file

    // Add the tags to preserve
    lines = resources.GenerateTagsConfigSection(logger, lines, resourceTags)
    // Add the resource types to preserve
    lines = GenerateResourceTypeConfigSection(logger, lines, resourceTypesToFilter)
    // Add the individual resources to preserve
    lines = generateResourceConfigSection(logger, lines, resourcesToPreserveByType)
    // Add any additional manual filters
    lines = addAdditionalFilters(logger, lines)
    // Overwrite the aws-nuke config file with the new contents
    writeLinesToFile(logger, generatedConfigFile, lines)

    // Run aws-nuke

    fmt.Println("\n\nRUNNING AWS-NUKE")
    fmt.Println()
    var cmd *exec.Cmd
    if noDryRun {
        cmd = exec.Command("bash", "-c", fmt.Sprintf("aws-nuke -c %v --no-dry-run", generatedConfigFile))
    } else {
        cmd = exec.Command("bash", "-c", fmt.Sprintf("aws-nuke -c %v", generatedConfigFile))
    }
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    _ = cmd.Run()
}
