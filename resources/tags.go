package resources

import (
	"awsnukeshield/helpers"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func GenerateTagsConfigSection(logger *zap.Logger, lines []string, tags []string) []string{
	var filterContents []string
    var indexToInsert int

	// Get the aws-nuke resource types
    awsNukeResourceTypesTemp, err := exec.Command("bash", "-c", "aws-nuke resource-types").Output()
    if err != nil {
        panic(err)
    }

	resourceTypesString := string(awsNukeResourceTypesTemp)
    awsNukeResourceTypes := strings.Split(resourceTypesString, "\n")

    index_of_filter_block := helpers.FindItem(lines, "filters:")
    if index_of_filter_block != -1 {
        // The config file already contains a filter block, so we must check it for each resource type and append the provided tags to each type if it already exists, else create a new type.
        for _, resourceType := range awsNukeResourceTypes {
            if len(resourceType) != 0 {
                    filterContents = []string{}

                    // Check if resource type already exists
                    index_of_resource_block := helpers.FindItem(lines, fmt.Sprintf("%s:", resourceType))
                    if index_of_resource_block != -1 {
                        indexToInsert = index_of_resource_block + 1
                    } else {
                        indexToInsert = index_of_filter_block + 1
                        filterContents = append(filterContents, fmt.Sprintf("      %s:", resourceType))
                    }
                    
                    // Add the tag filters to the temporary variable
                    for _, tag := range tags{
                        tagSplit := strings.Split(tag, ":")
                        filterContents = append(filterContents, fmt.Sprintf("        - property: tag:%s", tagSplit[0]))
                        filterContents = append(filterContents, fmt.Sprintf("          value: %s", tagSplit[1]))
                    }

                    // Insert the tag filters at the appropriate place in the file contents
                    lines = append(lines[:indexToInsert], append(filterContents, lines[indexToInsert:]...)...)
            }
        }

    } else {
        // The config file contains no filter block, so we create it and populate it with a block for every aws-nuke resource-type, where each block contains the provided tags

        for _, resourceType := range awsNukeResourceTypes {
            if len(resourceType) != 0 {
                    filterContents = append(filterContents, fmt.Sprintf("      %s:", resourceType))
                    for _, tag := range tags{
                        tagSplit := strings.Split(tag, ":")
                        filterContents = append(filterContents, fmt.Sprintf("        - property: tag:%s", tagSplit[0]))
                        filterContents = append(filterContents, fmt.Sprintf("          value: %s", tagSplit[1]))
                }
                
            }
        }
    
        // Create a filters section at the bottom of the file, and add the resource types with tag filters underneath
        lines = append(lines, "    filters:")
        lines = append(lines, filterContents...)
    }

    
	
    logger.Debug(fmt.Sprintf("Added tag blocks for every resource to the filters section: %v", lines))

    return lines
}