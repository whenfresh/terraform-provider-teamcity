package teamcity

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/dghubble/sling"

	loghttp "github.com/motemen/go-loghttp"
	// Enable HTTP log tracing
	_ "github.com/motemen/go-loghttp/global"
)

//DebugRequests toggle to enable tracing requests to stdout
var DebugRequests = false

//DebugResponses tottle to enable tracing responses to stdout
var DebugResponses = false

func init() {
	loghttp.DefaultTransport.LogRequest = func(resp *http.Request) {
		if DebugRequests {
			debug(httputil.DumpRequest(resp, true))
		}
	}
	loghttp.DefaultTransport.LogResponse = func(resp *http.Response) {
		if DebugResponses {
			debug(httputil.DumpResponse(resp, true))
		}
	}
}

//Client represents the base for connecting to TeamCity
type Client struct {
	userName, password, address string
	baseURI                     string

	HTTPClient   *http.Client
	RetryTimeout time.Duration

	commonBase *sling.Sling

	Projects   *ProjectService
	BuildTypes *BuildTypeService
	Server     *ServerService
	VcsRoots   *VcsRootService
	Parameters *ParameterService
}

// New creates a new client for server address specified at TEAMCITY_ADDR environment variable
func New(userName, password string, httpClient *http.Client) (*Client, error) {
	address := os.Getenv("TEAMCITY_ADDR")
	if address == "" {
		return nil, fmt.Errorf("TEAMCITY_ADDR environment variable not set, specify address explicit by setting the variable or using NewWithAddress constructor")
	}

	return newClientInstance(userName, password, address, httpClient)
}

// NewWithAddress creates a new client by using the explicit server address from the parameter
func NewWithAddress(userName, password, address string, httpClient *http.Client) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}
	return newClientInstance(userName, password, address, httpClient)
}

func newClientInstance(userName, password, address string, httpClient *http.Client) (*Client, error) {
	sharedClient := sling.New().Base(address+"/httpAuth/app/rest/").
		SetBasicAuth(userName, password).
		Set("Accept", "application/json")

	return &Client{
		userName:   userName,
		password:   password,
		address:    address,
		HTTPClient: httpClient,
		commonBase: sharedClient,
		Projects:   newProjectService(sharedClient.New(), httpClient),
		BuildTypes: newBuildTypeService(sharedClient.New(), httpClient),
		Server:     newServerService(sharedClient.New()),
		VcsRoots:   newVcsRootService(sharedClient.New(), httpClient),
	}, nil
}

//AgentRequirementService returns a service to manage agent requirements for a build configuration with given id
func (c *Client) AgentRequirementService(id string) *AgentRequirementService {
	return newAgentRequirementService(id, c.HTTPClient, c.commonBase.New())
}

//BuildFeatureService returns a service to manage agent requirements for a build configuration with given id
func (c *Client) BuildFeatureService(id string) *BuildFeatureService {
	return newBuildFeatureService(id, c.HTTPClient, c.commonBase.New())
}

//ProjectParameterService returns a parameter service that operates parameters for the project with given id
func (c *Client) ProjectParameterService(id string) *ParameterService {
	return &ParameterService{
		base: c.commonBase.New().Path(fmt.Sprintf("projects/%s/", LocatorID(id))),
	}
}

//BuildTypeParameterService returns a parameter service that operates parameters for the build configuration with given id
func (c *Client) BuildTypeParameterService(id string) *ParameterService {
	return &ParameterService{
		base: c.commonBase.New().Path(fmt.Sprintf("buildTypes/%s/", LocatorID(id))),
	}
}

//DependencyService returns a service to manage snapshot and artifact dependencies for a build configuration with given id
func (c *Client) DependencyService(id string) *DependencyService {
	return NewDependencyService(id, c.HTTPClient, c.commonBase.New())
}

//TriggerService returns a service to manage build triggers for a build configuration with given id
func (c *Client) TriggerService(buildTypeID string) *TriggerService {
	return newTriggerService(buildTypeID, c.HTTPClient, c.commonBase.New())
}

// Validate tests if the client is properly configured and can be used
func (c *Client) Validate() (bool, error) {
	response, err := c.commonBase.Get("server").ReceiveSuccess(nil)

	if err != nil {
		return false, err
	}

	if response.StatusCode != 200 && response.StatusCode != 403 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("API error %s: %s", response.Status, body)
	}

	return true, nil
}

type textPlainBodyProvider struct {
	payload interface{}
}

func (p textPlainBodyProvider) ContentType() string {
	return "text/plain; charset=utf-8"
}

func (p textPlainBodyProvider) Body() (io.Reader, error) {
	return strings.NewReader(p.payload.(string)), nil
}

func debug(data []byte, err error) {
	if err == nil {
		fmt.Printf("%s\n\n", data)
	} else {
		log.Printf("[ERROR] %s\n\n", err)
	}
}
