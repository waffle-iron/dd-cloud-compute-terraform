package ddcloud

import (
	"fmt"
	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	resourceKeyFirewallRuleNetworkDomainID             = "networkdomain"
	resourceKeyFirewallRuleName                        = "name"
	resourceKeyFirewallRuleAction                      = "action"
	resourceKeyFirewallRuleEnabled                     = "enabled"
	resourceKeyFirewallRulePlacement                   = "placement"
	resourceKeyFirewallRulePlacementRelativeToRuleName = "placement_relative_to"
	resourceKeyFirewallRuleIPVersion                   = "ip_version"
	resourceKeyFirewallRuleProtocol                    = "protocol"
	resourceKeyFirewallRuleSourceAddress               = "source_address"
	resourceKeyFirewallRuleSourceNetwork               = "source_network"
	resourceKeyFirewallRuleSourcePort                  = "source_port"
	resourceKeyFirewallRuleDestinationAddress          = "destination_address"
	resourceKeyFirewallRuleDestinationNetwork          = "destination_network"
	resourceKeyFirewallRuleDestinationPort             = "destination_port"
	resourceCreateTimeoutFirewallRule                  = 30 * time.Minute
	resourceUpdateTimeoutFirewallRule                  = 10 * time.Minute
	resourceDeleteTimeoutFirewallRule                  = 15 * time.Minute
)

const matchAny = "any"
const computedPropertyDescription = "<computed>"

func resourceFirewallRule() *schema.Resource {
	return &schema.Resource{
		Create: resourceFirewallRuleCreate,
		Read:   resourceFirewallRuleRead,
		Update: resourceFirewallRuleUpdate,
		Delete: resourceFirewallRuleDelete,

		Schema: map[string]*schema.Schema{
			resourceKeyFirewallRuleNetworkDomainID: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The Id of the network domain to which the firewall rule applies",
			},
			resourceKeyFirewallRuleName: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "A name for the firewall rule",
			},
			resourceKeyFirewallRuleAction: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The action performed by the firewall rule",
				StateFunc: func(value interface{}) string {
					action := value.(string)

					return normalizeFirewallRuleAction(action)
				},
			},
			resourceKeyFirewallRuleEnabled: &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Is the firewall rule enabled",
			},
			resourceKeyFirewallRulePlacement: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Default:     "first",
				Description: "Where in the firewall ACL this particular rule will be created",
			},
			resourceKeyFirewallRulePlacementRelativeToRuleName: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Default:     nil,
				Description: "When placement is 'before' or 'after', specifies the name of the firewall rule to which the placement instruction refers",
			},
			resourceKeyFirewallRuleIPVersion: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The IP version to which the firewall rule applies",
			},
			resourceKeyFirewallRuleProtocol: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The protocol to which the rule applies",
			},
			resourceKeyFirewallRuleSourceAddress: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The source IP address to be matched by the rule",
				ConflictsWith: []string{
					resourceKeyFirewallRuleSourceNetwork,
				},
			},
			resourceKeyFirewallRuleSourceNetwork: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The source IP network to be matched by the rule",
				ConflictsWith: []string{
					resourceKeyFirewallRuleSourceAddress,
				},
			},
			resourceKeyFirewallRuleSourcePort: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The source port to be matched by the rule",
			},
			resourceKeyFirewallRuleDestinationAddress: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The destination IP address to be matched by the rule",
				ConflictsWith: []string{
					resourceKeyFirewallRuleDestinationNetwork,
				},
			},
			resourceKeyFirewallRuleDestinationNetwork: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The destination IP network to be matched by the rule",
				ConflictsWith: []string{
					resourceKeyFirewallRuleDestinationAddress,
				},
			},
			resourceKeyFirewallRuleDestinationPort: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The destination port to be matched by the rule",
			},
		},
	}
}

// Create a firewall rule resource.
func resourceFirewallRuleCreate(data *schema.ResourceData, provider interface{}) error {
	var err error

	propertyHelper := propertyHelper(data)

	configuration := &compute.FirewallRuleConfiguration{
		Name: data.Get(resourceKeyFirewallRuleName).(string),
		Action: normalizeFirewallRuleAction(
			data.Get(resourceKeyFirewallRuleAction).(string),
		),
		Placement: compute.FirewallRulePlacement{
			Position: strings.ToUpper(
				data.Get(resourceKeyFirewallRulePlacement).(string),
			),
			RelativeToRuleName: propertyHelper.GetOptionalString(
				resourceKeyFirewallRulePlacementRelativeToRuleName, false,
			),
		},
		Enabled:         data.Get(resourceKeyFirewallRuleEnabled).(bool),
		NetworkDomainID: data.Get(resourceKeyFirewallRuleNetworkDomainID).(string),
		IPVersion: strings.ToUpper(
			data.Get(resourceKeyFirewallRuleIPVersion).(string),
		),
		Protocol: strings.ToUpper(
			data.Get(resourceKeyFirewallRuleProtocol).(string),
		),
	}

	configuration.Action = data.Get(resourceKeyFirewallRuleAction).(string)

	err = configureSourceScope(propertyHelper, configuration)
	if err != nil {
		return err
	}
	err = configureDestinationScope(propertyHelper, configuration)
	if err != nil {
		return err
	}

	log.Printf("Create firewall rule '%s' in network domain '%s'.", configuration.Name, configuration.NetworkDomainID)
	log.Printf("Firewall rule configuration: '%#v'", configuration)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	domainLock := providerState.GetDomainLock(configuration.NetworkDomainID, "resourceFirewallRuleCreate('%s')", configuration.Name)
	domainLock.Lock()
	defer domainLock.Unlock()

	ruleID, err := apiClient.CreateFirewallRule(*configuration)
	if err != nil {
		return err
	}

	data.SetId(ruleID)

	_, err = apiClient.WaitForDeploy(compute.ResourceTypeFirewallRule, ruleID, resourceCreateTimeoutFirewallRule)

	return err
}

// Read a firewall rule resource.
func resourceFirewallRuleRead(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	networkDomainID := data.Get(resourceKeyFirewallRuleNetworkDomainID).(string)

	log.Printf("Read firewall rule '%s' in network domain '%s'.", id, networkDomainID)

	apiClient := provider.(*providerState).Client()

	rule, err := apiClient.GetFirewallRule(id)
	if err != nil {
		return err
	}
	if rule == nil {
		log.Printf("Firewall rule '%s' has been deleted.", id)

		data.SetId("")

		return nil
	}

	data.Set(resourceKeyFirewallRuleEnabled, rule.Enabled)

	return nil
}

// Update a firewall rule resource.
func resourceFirewallRuleUpdate(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	name := data.Get(resourceKeyFirewallRuleName).(string)
	networkDomainID := data.Get(resourceKeyFirewallRuleNetworkDomainID).(string)

	log.Printf("Update firewall rule '%s' in network domain '%s'.", id, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	domainLock := providerState.GetDomainLock(networkDomainID, "resourceFirewallRuleUpdate(id = '%s', name = '%s')", id, name)
	domainLock.Lock()
	defer domainLock.Unlock()

	if data.HasChange(resourceKeyFirewallRuleEnabled) {
		enable := data.Get(resourceKeyFirewallRuleEnabled).(bool)

		if enable {
			log.Printf("Enabling firewall rule '%s'...", id)
		} else {
			log.Printf("Disabling firewall rule '%s'...", id)
		}

		err := apiClient.EditFirewallRule(id, enable)
		if err != nil {
			return err
		}

		log.Printf("Updated configuration for firewall rule '%s'.", id)
	}

	return nil
}

// Delete a firewall rule resource.
func resourceFirewallRuleDelete(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	name := data.Get(resourceKeyFirewallRuleName).(string)
	networkDomainID := data.Get(resourceKeyFirewallRuleNetworkDomainID).(string)

	log.Printf("Delete firewall rule '%s' in network domain '%s'.", id, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	domainLock := providerState.GetDomainLock(networkDomainID, "resourceFirewallRuleDelete(id = '%s', name = '%s')", id, name)
	domainLock.Lock()
	defer domainLock.Unlock()

	err := apiClient.DeleteFirewallRule(id)
	if err != nil {
		return err
	}

	return apiClient.WaitForDelete(compute.ResourceTypeFirewallRule, id, resourceDeleteTimeoutFirewallRule)
}

func configureSourceScope(propertyHelper resourcePropertyHelper, configuration *compute.FirewallRuleConfiguration) error {
	sourceAddress := propertyHelper.GetOptionalString(resourceKeyFirewallRuleSourceAddress, false)
	sourceNetwork := propertyHelper.GetOptionalString(resourceKeyFirewallRuleSourceNetwork, false)
	sourcePort, err := parseFirewallPort(
		propertyHelper.GetOptionalString(resourceKeyFirewallRuleSourcePort, false),
	)
	if err != nil {
		return err
	}
	if sourceAddress != nil {
		log.Printf("Rule will match source address '%s'.", *sourceAddress)
		configuration.MatchSourceAddressAndPort(*sourceAddress, sourcePort) // Port ranges not supported yet.
	} else if sourceNetwork != nil {
		log.Printf("Rule will match source network '%s'.", *sourceNetwork)

		baseAddress, prefixSize, ok := parseNetworkAndPrefix(*sourceNetwork)
		if !ok {
			return fmt.Errorf("Source network '%s' for firewall rule '%s' is invalid (must be 'BaseAddress/PrefixSize')",
				*sourceNetwork,
				propertyHelper.data.Get(resourceKeyFirewallRuleName).(string),
			)
		}

		configuration.MatchSourceNetworkAndPort(baseAddress, prefixSize, sourcePort)
	} else if sourcePort != nil {
		log.Printf("Rule will match any source address with port %d.", *sourcePort)
		configuration.MatchAnySourceAddress(sourcePort)
	} else {
		log.Print("Rule will match any source address and port.")
		configuration.MatchAnySource()
	}

	return nil
}

func configureDestinationScope(propertyHelper resourcePropertyHelper, configuration *compute.FirewallRuleConfiguration) error {
	destinationNetwork := propertyHelper.GetOptionalString(resourceKeyFirewallRuleDestinationNetwork, false)
	destinationAddress := propertyHelper.GetOptionalString(resourceKeyFirewallRuleDestinationAddress, false)

	destinationPort, err := parseFirewallPort(
		propertyHelper.GetOptionalString(resourceKeyFirewallRuleDestinationPort, false),
	)
	if err != nil {
		return err
	}
	if destinationAddress != nil {
		log.Printf("Rule will match destination address '%s'.", *destinationAddress)
		configuration.MatchDestinationAddressAndPort(*destinationAddress, destinationPort) // Port ranges not supported yet.
	} else if destinationNetwork != nil {
		log.Printf("Rule will match destination network '%s'.", *destinationNetwork)

		baseAddress, prefixSize, ok := parseNetworkAndPrefix(*destinationNetwork)
		if !ok {
			return fmt.Errorf("Source network '%s' for firewall rule '%s' is invalid (must be 'BaseAddress/PrefixSize')",
				*destinationNetwork,
				propertyHelper.data.Get(resourceKeyFirewallRuleName).(string),
			)
		}

		configuration.MatchDestinationNetworkAndPort(baseAddress, prefixSize, destinationPort)
	} else if destinationPort != nil {
		log.Printf("Rule will match any destination address with port %d.", *destinationPort)
		configuration.MatchAnyDestinationAddress(destinationPort)
	} else {
		log.Print("Rule will match any destination address and port.")
		configuration.MatchAnyDestination()
	}

	return nil
}

func normalizeFirewallRuleAction(action string) string {
	switch strings.ToLower(action) {
	case "accept":
		return compute.FirewallRuleActionAccept

	case "accept_decisively":
		return compute.FirewallRuleActionAccept

	case "allow":
		return compute.FirewallRuleActionAccept

	case "drop":
		return compute.FirewallRuleActionDrop

	case "deny":
		return compute.FirewallRuleActionDrop

	default:
		return action
	}
}

func parseFirewallPort(port *string) (*int, error) {
	if port == nil || *port == "any" {
		return nil, nil
	}

	parsedPort, err := strconv.Atoi(*port)
	if err != nil {
		return nil, err
	}

	return &parsedPort, nil
}

func parsePortRange(portRange *string) (beginPort string, endPort *string) {
	ports := strings.Split(*portRange, "-")
	beginPort = strings.TrimSpace(ports[0])

	if len(ports) == 1 {
		return
	}

	ports[1] = strings.TrimSpace(ports[1])
	endPort = &ports[1]

	return
}

func parseNetworkAndPrefix(networkAndPrefix string) (baseAddress string, prefixSize int, ok bool) {
	networkComponents := strings.Split(networkAndPrefix, "/")
	if len(networkComponents) != 2 {
		return
	}

	baseAddress = networkComponents[0]
	prefixSize, err := strconv.Atoi(networkComponents[1])
	if err != nil {
		return
	}

	ok = true

	return
}
