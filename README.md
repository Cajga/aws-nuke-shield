# aws-nuke-shield
Wrap rebuy-de/aws-nuke (https://github.com/rebuy-de/aws-nuke) to allow the preservation of resources which meet certain conditions.

The tool aws-nuke allows for deleting many resources from a target AWS account, and uses a config file format which supports filtering resources based on many resource properties. 
However there are certain qualities (such as being a child of a certain CloudFormation stack) which are not currently supported. This interctive tool aims to fill this gap. 

## The currently supported use cases
* Resources which are children of a certain CloudFormation (CFN) Stack
* Resources with certain tags

## How the tool works
* You provide a list of regular expressions for matching CFN stacks [optional], and a list of tags as key:value pairs [optional]
* The tool queries the AWS API for the stacks matching any of the regexes, and then calls ListStackResources on each stack to get its child resources
* The tool populates an aws-nuke formatted config file with the desired tags and identifiers of the child resources. By default the source is `example-nuke-config.yaml`, but a custom file can be provided with the `config` parameter. The source file is NOT overwritten, Shield creates a new version with `-shield-generated` appended to the name
* The tool then runs aws-nuke using the generated config file
* Certain resources are not filterable using the standard aws-nuke filter mechanism. For these, you can either:
  * add them manually to the file before running the tool; in this case the tool will make its modifications to the file as usual, preserving the preexisting content
  * add them to the function `addAdditionalFilters` in main.go; in this case the script will add the lines programmatically. This function contains some preexisting lines for the currently known use case

## Caution!
Given this tool is a wrapper for aws-nuke, the same disclaimers apply. Aws-nuke is a very destructive tool. We strongly advise you to not run this application on any AWS account, where you cannot afford to lose all resources.
Although this wrapper should preserve all the resources which meet the given conditions, there are certain cases where a resource will not be added to the config file. These cases should all be detailed in the output. It is therefore important to double-check that you are happy with what aws-nuke is planning to delete before proceeding with the deletion.

## Usage
1) Run `go build` from this directory to build the application `awsnukeshield`
2) Ensure that `example-nuke-config.yml` has the correct regions and account details
3) Identify the regexes to use for CloudFormation Stacks, as well as tags and entire resource types which want preserving
4) Formulate the command accordingly:
e.g. `./awsnukewrapper -regexes ".*StackSet-AWS.*" -tags "terraform:true" -preserve-resource-types "GuardDutyDetector","CloudTrailTrail","ConfigServiceConfigRule","SecurityHub"`
will preserve any child resources of CFN stacks matching `.*StackSet-AWS.*`, any resources with the tag `terraform:true`, and all resources of type `GuardDutyDetector`, `CloudTrailTrail`, `ConfigServiceConfigRule`, and `SecurityHub`. Note that these arguments all operate independently, i.e. a resource meeting any one or more of these criteria will be preserved
5) Run the command, follow any prompts for mapping resource types, then confirm the running of aws-nuke
6) **Important**: by default, Shield will run aws-nuke in `dry-run` mode, which only shows what would be deleted, without performing the operation. Once happy with this, run Shield again, this time with the `-no-dry-run` flag. Take care, as should you choose to proceed, the chosen account will be nuked

## Limitations
* A resource can only be deleted if currently supported by aws-nuke. This means that certain resources will not be added to the config file for preservation, despite being children of identified CFN stacks. This should not result in their deletion, as aws-nuke will of course only be able to delete resources which it supports. In addition, provided aws-nuke keeps the command `aws-nuke resource-types` up-to-date, Shield will automatically begin adding such resources for preservation to the config file should they later become supported by aws-nuke
* Due to the mismatch between resource type names returned by the AWS APIs used by Shield and the names accepted by aws-nuke, Shield currently requires manual user input to determine certain mappings
* During use, further resource types may be discovered which are added to the config file but not successfully filtered. For each case, one should identify a suitable parameter to use for filtering, and either add to the `customProperties` variable or as a last resort add it manually to the template config file or programmatically via the addAdditionalFilters function

## Contribute
You can contribute to the project by forking this repository, making your changes and creating a Pull Request against our repository. If you are unsure how to solve a problem or have other questions about a contributions, please create a GitHub issue.