// Copyright 2018 Bull S.A.S. Atos Technologies - Bull, Rue Jean Jaures, B.P.68, 78340, Les Clayes-sous-Bois, France.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ystia/yorc/config"
	"github.com/ystia/yorc/helper/collections"
	"github.com/ystia/yorc/log"
	"github.com/ystia/yorc/server"
)

func init() {
	RootCmd.AddCommand(serverCmd)

	// Get the CLI args
	args := os.Args

	serverInitExtraFlags(args)
	setConfig()
	cobra.OnInitialize(initConfig)
}

const (
	environmentVariablePrefix = "YORC"
)

var ansibleConfiguration = map[string]interface{}{
	"ansible.use_openssh":                false,
	"ansible.debug":                      false,
	"ansible.connection_retries":         5,
	"ansible.operation_remote_base_dir":  ".yorc",
	"ansible.keep_operation_remote_path": config.DefaultKeepOperationRemotePath,
}

var consulConfiguration = map[string]interface{}{
	"consul.address":                "",
	"consul.token":                  "anonymous",
	"consul.datacenter":             "dc1",
	"consul.key_file":               "",
	"consul.cert_file":              "",
	"consul.ca_cert":                "",
	"consul.ca_path":                "",
	"consul.ssl":                    false,
	"consul.ssl_verify":             true,
	"consul.publisher_max_routines": config.DefaultConsulPubMaxRoutines,
}

var cfgFile string

var resolvedServerExtraParams []*serverExtraParams

type serverExtraParams struct {
	argPrefix   string
	envPrefix   string
	viperPrefix string
	viperNames  []string
	subSplit    int
	storeFn     serverExtraParamStoreFn
	readConfFn  serverExtraParamReadConf
}

type serverExtraParamStoreFn func(cfg *config.Configuration, param string)
type serverExtraParamReadConf func(cfg *config.Configuration)

var serverCmd = &cobra.Command{
	Use:          "server",
	Short:        "Perform the server command",
	Long:         `Perform the server command`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		configuration := getConfig()
		log.Debugf("Configuration :%+v", configuration)
		shutdownCh := make(chan struct{})
		return server.RunServer(configuration, shutdownCh)
	},
}

func serverInitExtraFlags(args []string) {
	resolvedServerExtraParams = []*serverExtraParams{
		&serverExtraParams{
			argPrefix:   "infrastructure_",
			envPrefix:   "YORC_INFRA_",
			viperPrefix: "infrastructures.",
			viperNames:  make([]string, 0),
			subSplit:    1,
			storeFn:     addServerExtraInfraParams,
			readConfFn:  readInfraViperConfig,
		},
		&serverExtraParams{
			argPrefix:   "vault_",
			envPrefix:   "YORC_VAULT_",
			viperPrefix: "vault.",
			viperNames:  make([]string, 0),
			storeFn:     addServerExtraVaultParam,
			readConfFn:  readVaultViperConfig,
		},
	}

	for _, sep := range resolvedServerExtraParams {
		for i := range args {
			if strings.HasPrefix(args[i], "--"+sep.argPrefix) {
				var viperName, flagName string
				if strings.ContainsRune(args[i], '=') {
					// Handle the syntax --infrastructure_xxx_yyy = value
					flagParts := strings.Split(args[i], "=")
					flagName = strings.TrimLeft(flagParts[0], "-")
					viperName = strings.Replace(strings.Replace(flagName, sep.argPrefix, sep.viperPrefix, 1), "_", ".", sep.subSplit)
					if len(flagParts) == 1 {
						// Boolean flag
						serverCmd.PersistentFlags().Bool(flagName, false, "")
						viper.SetDefault(viperName, false)
					} else {
						serverCmd.PersistentFlags().String(flagName, "", "")
						viper.SetDefault(viperName, "")
					}
				} else {
					// Handle the syntax --infrastructure_xxx_yyy value
					flagName = strings.TrimLeft(args[i], "-")
					viperName = strings.Replace(strings.Replace(flagName, sep.argPrefix, sep.viperPrefix, 1), "_", ".", sep.subSplit)
					if len(args) > i+1 && !strings.HasPrefix(args[i+1], "--") {
						serverCmd.PersistentFlags().String(flagName, "", "")
						viper.SetDefault(viperName, "")
					} else {
						// Boolean flag
						serverCmd.PersistentFlags().Bool(flagName, false, "")
						viper.SetDefault(viperName, false)
					}
				}
				// Add viper flag
				viper.BindPFlag(viperName, serverCmd.PersistentFlags().Lookup(flagName))
				sep.viperNames = append(sep.viperNames, viperName)
			}
		}
		for _, envVar := range os.Environ() {
			if strings.HasPrefix(envVar, sep.envPrefix) {
				envVarParts := strings.SplitN(envVar, "=", 2)
				viperName := strings.ToLower(strings.Replace(strings.Replace(envVarParts[0], sep.envPrefix, sep.viperPrefix, 1), "_", ".", sep.subSplit))
				viper.BindEnv(viperName, envVarParts[0])
				if !collections.ContainsString(sep.viperNames, viperName) {
					sep.viperNames = append(sep.viperNames, viperName)
				}
			}
		}
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		log.Println("Can't use config file:", err)
	}

	// Deprecate Ansible and Consul flat keys if they are defined in
	// configuration
	deprecateFlatKeys(ansibleConfiguration, "ansible")
	deprecateFlatKeys(consulConfiguration, "consul")
}

func setConfig() {

	//Flags definition for Yorc server
	serverCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is /etc/yorc/config.yorc.json)")
	serverCmd.PersistentFlags().String("plugins_directory", config.DefaultPluginDir, "The name of the plugins directory of the Yorc server")
	serverCmd.PersistentFlags().StringP("working_directory", "w", "", "The name of the working directory of the Yorc server")
	serverCmd.PersistentFlags().Int("workers_number", config.DefaultWorkersNumber, "Number of workers in the Yorc server. If not set the default value will be used")
	serverCmd.PersistentFlags().Duration("graceful_shutdown_timeout", config.DefaultServerGracefulShutdownTimeout, "Timeout to  wait for a graceful shutdown of the Yorc server. After this delay the server immediately exits.")
	serverCmd.PersistentFlags().StringP("resources_prefix", "x", "", "Prefix created resources (like Computes and so on)")
	serverCmd.PersistentFlags().Duration("wf_step_graceful_termination_timeout", config.DefaultWfStepGracefulTerminationTimeout, "Timeout to wait for a graceful termination of a workflow step during concurrent workflow step failure. After this delay the step is set on error.")

	// Flags definition for Yorc HTTP REST API
	serverCmd.PersistentFlags().Int("http_port", config.DefaultHTTPPort, "Port number for the Yorc HTTP REST API. If omitted or set to '0' then the default port number is used, any positive integer will be used as it, and finally any negative value will let use a random port.")
	serverCmd.PersistentFlags().String("http_address", config.DefaultHTTPAddress, "Listening address for the Yorc HTTP REST API.")
	serverCmd.PersistentFlags().String("key_file", "", "File path to a PEM-encoded private key. The key is used to enable SSL for the Yorc HTTP REST API. This must be provided along with cert_file. If one of key_file or cert_file is not provided then SSL is disabled.")
	serverCmd.PersistentFlags().String("cert_file", "", "File path to a PEM-encoded certificate. The certificate is used to enable SSL for the Yorc HTTP REST API. This must be provided along with key_file. If one of key_file or cert_file is not provided then SSL is disabled.")

	//Flags definition for Consul
	serverCmd.PersistentFlags().StringP("consul_address", "", "", "Address of the HTTP interface for Consul (format: <host>:<port>)")
	serverCmd.PersistentFlags().StringP("consul_token", "t", "", "The token by default")
	serverCmd.PersistentFlags().StringP("consul_datacenter", "d", "", "The datacenter of Consul node")
	serverCmd.PersistentFlags().String("consul_key_file", "", "The key file to use for talking to Consul over TLS")
	serverCmd.PersistentFlags().String("consul_cert_file", "", "The cert file to use for talking to Consul over TLS")
	serverCmd.PersistentFlags().String("consul_ca_cert", "", "CA cert to use for talking to Consul over TLS")
	serverCmd.PersistentFlags().String("consul_ca_path", "", "Path to a directory of CA certs to use for talking to Consul over TLS")
	serverCmd.PersistentFlags().Bool("consul_ssl", false, "Whether or not to use HTTPS")
	serverCmd.PersistentFlags().Bool("consul_ssl_verify", true, "Whether or not to disable certificate checking")

	serverCmd.PersistentFlags().Int("consul_publisher_max_routines", config.DefaultConsulPubMaxRoutines, "Maximum number of parallelism used to store TOSCA definitions in Consul. If you increase the default value you may need to tweak the ulimit max open files. If set to 0 or less the default value will be used")

	serverCmd.PersistentFlags().Bool("ansible_use_openssh", false, "Prefer OpenSSH over Paramiko a Python implementation of SSH (the default) to provision remote hosts")
	serverCmd.PersistentFlags().Bool("ansible_debug", false, "Prints massive debug information from Ansible")
	serverCmd.PersistentFlags().Int("ansible_connection_retries", 5, "Number of retries in case of Ansible SSH connection failure")
	serverCmd.PersistentFlags().String("operation_remote_base_dir", ".yorc", "Name of the temporary directory used by Ansible on the nodes")
	serverCmd.PersistentFlags().Bool("keep_operation_remote_path", config.DefaultKeepOperationRemotePath, "Define wether the path created to store artifacts on the nodes will be removed at the end of workflow executions.")

	//Bind Consul persistent flags
	for key := range consulConfiguration {
		viper.BindPFlag(key, serverCmd.PersistentFlags().Lookup(toFlatKey(key)))
	}

	//Bind Flags for Yorc server
	viper.BindPFlag("working_directory", serverCmd.PersistentFlags().Lookup("working_directory"))
	viper.BindPFlag("plugins_directory", serverCmd.PersistentFlags().Lookup("plugins_directory"))
	viper.BindPFlag("workers_number", serverCmd.PersistentFlags().Lookup("workers_number"))
	viper.BindPFlag("server_graceful_shutdown_timeout", serverCmd.PersistentFlags().Lookup("graceful_shutdown_timeout"))
	viper.BindPFlag("resources_prefix", serverCmd.PersistentFlags().Lookup("resources_prefix"))
	viper.BindPFlag("wf_step_graceful_termination_timeout", serverCmd.PersistentFlags().Lookup("wf_step_graceful_termination_timeout"))

	//Bind Flags Yorc HTTP REST API
	viper.BindPFlag("http_port", serverCmd.PersistentFlags().Lookup("http_port"))
	viper.BindPFlag("http_address", serverCmd.PersistentFlags().Lookup("http_address"))
	viper.BindPFlag("cert_file", serverCmd.PersistentFlags().Lookup("cert_file"))
	viper.BindPFlag("key_file", serverCmd.PersistentFlags().Lookup("key_file"))

	//Bind Ansible persistent flags
	for key := range ansibleConfiguration {
		viper.BindPFlag(key, serverCmd.PersistentFlags().Lookup(toFlatKey(key)))
	}

	//Environment Variables
	viper.SetEnvPrefix(environmentVariablePrefix)
	viper.AutomaticEnv() // read in environment variables that match
	viper.BindEnv("working_directory")
	viper.BindEnv("plugins_directory")
	viper.BindEnv("server_graceful_shutdown_timeout")
	viper.BindEnv("workers_number")
	viper.BindEnv("http_port")
	viper.BindEnv("http_address")
	viper.BindEnv("key_file")
	viper.BindEnv("cert_file")
	viper.BindEnv("resources_prefix")

	//Bind Consul environment variables flags
	for key := range consulConfiguration {
		viper.BindEnv(key, toEnvVar(key))
	}

	viper.BindEnv("wf_step_graceful_termination_timeout")

	//Bind Ansible environment variables flags
	for key := range ansibleConfiguration {
		viper.BindEnv(key, toEnvVar(key))
	}

	//Setting Defaults
	viper.SetDefault("working_directory", "work")
	viper.SetDefault("server_graceful_shutdown_timeout", config.DefaultServerGracefulShutdownTimeout)
	viper.SetDefault("plugins_directory", config.DefaultPluginDir)
	viper.SetDefault("http_port", config.DefaultHTTPPort)
	viper.SetDefault("http_address", config.DefaultHTTPAddress)
	viper.SetDefault("resources_prefix", "yorc-")
	viper.SetDefault("workers_number", config.DefaultWorkersNumber)
	viper.SetDefault("wf_step_graceful_termination_timeout", config.DefaultWfStepGracefulTerminationTimeout)

	// Consul configuration default settings
	for key, value := range consulConfiguration {
		viper.SetDefault(key, value)
	}

	// Ansible configuration default settings
	for key, value := range ansibleConfiguration {
		viper.SetDefault(key, value)
	}

	//Configuration file directories
	viper.SetConfigName("config.yorc") // name of config file (without extension)
	viper.AddConfigPath("/etc/yorc/")  // adding home directory as first search path
	viper.AddConfigPath(".")

}

func getConfig() config.Configuration {
	configuration := config.Configuration{}
	configuration.Ansible.UseOpenSSH = viper.GetBool("ansible.use_openssh")
	configuration.Ansible.DebugExec = viper.GetBool("ansible.debug")
	configuration.Ansible.ConnectionRetries = viper.GetInt("ansible.connection_retries")
	configuration.Ansible.OperationRemoteBaseDir = viper.GetString("ansible.operation_remote_base_dir")
	configuration.Ansible.KeepOperationRemotePath = viper.GetBool("ansible.keep_operation_remote_path")
	configuration.WorkingDirectory = viper.GetString("working_directory")
	configuration.PluginsDirectory = viper.GetString("plugins_directory")
	configuration.WorkersNumber = viper.GetInt("workers_number")
	configuration.HTTPPort = viper.GetInt("http_port")
	configuration.HTTPAddress = viper.GetString("http_address")
	configuration.CertFile = viper.GetString("cert_file")
	configuration.KeyFile = viper.GetString("key_file")
	configuration.ResourcesPrefix = viper.GetString("resources_prefix")
	configuration.Consul.Address = viper.GetString("consul.address")
	configuration.Consul.Datacenter = viper.GetString("consul.datacenter")
	configuration.Consul.Token = viper.GetString("consul.token")
	configuration.Consul.PubMaxRoutines = viper.GetInt("consul.publisher_max_routines")
	configuration.Consul.Key = viper.GetString("consul.key_file")
	configuration.Consul.Cert = viper.GetString("consul.cert_file")
	configuration.Consul.CA = viper.GetString("consul.ca_cert")
	configuration.Consul.CAPath = viper.GetString("consul.ca_path")
	configuration.Consul.SSL = viper.GetBool("consul.ssl")
	configuration.Consul.SSLVerify = viper.GetBool("consul.ssl_verify")
	configuration.ServerGracefulShutdownTimeout = viper.GetDuration("server_graceful_shutdown_timeout")
	configuration.WfStepGracefulTerminationTimeout = viper.GetDuration("wf_step_graceful_termination_timeout")
	configuration.Infrastructures = make(map[string]config.DynamicMap)
	configuration.Vault = make(config.DynamicMap)

	for _, sep := range resolvedServerExtraParams {
		sep.readConfFn(&configuration)
		for _, infraParam := range sep.viperNames {
			sep.storeFn(&configuration, infraParam)
		}
	}
	configuration.Telemetry.StatsdAddress = viper.GetString("telemetry.statsd_address")
	configuration.Telemetry.StatsiteAddress = viper.GetString("telemetry.statsite_address")
	configuration.Telemetry.ServiceName = viper.GetString("telemetry.service_name")
	configuration.Telemetry.PrometheusEndpoint = viper.GetBool("telemetry.expose_prometheus_endpoint")
	configuration.Telemetry.DisableHostName = viper.GetBool("telemetry.disable_hostname")
	configuration.Telemetry.DisableGoRuntimeMetrics = viper.GetBool("telemetry.disable_go_runtime_metrics")

	return configuration
}

func readInfraViperConfig(cfg *config.Configuration) {
	infras := viper.GetStringMap("infrastructures")
	for infraName, infraConf := range infras {
		infraConfMap, ok := infraConf.(map[string]interface{})
		if !ok {
			log.Fatalf("Invalid configuration format for infrastructure %q", infraName)
		}
		if cfg.Infrastructures[infraName] == nil {
			cfg.Infrastructures[infraName] = make(config.DynamicMap)
		}
		for k, v := range infraConfMap {
			cfg.Infrastructures[infraName].Set(k, v)
		}
	}
}

func readVaultViperConfig(cfg *config.Configuration) {
	vaultCfg := viper.GetStringMap("vault")
	for k, v := range vaultCfg {
		cfg.Vault.Set(k, v)
	}
}

func addServerExtraInfraParams(cfg *config.Configuration, infraParam string) {
	if cfg.Infrastructures == nil {
		cfg.Infrastructures = make(map[string]config.DynamicMap)
	}
	paramParts := strings.Split(infraParam, ".")
	value := viper.Get(infraParam)
	params, ok := cfg.Infrastructures[paramParts[1]]
	if !ok {
		params = make(config.DynamicMap)
		cfg.Infrastructures[paramParts[1]] = params
	}
	params.Set(paramParts[2], value)
}

func addServerExtraVaultParam(cfg *config.Configuration, vaultParam string) {
	paramParts := strings.Split(vaultParam, ".")
	value := viper.Get(vaultParam)
	cfg.Vault.Set(paramParts[1], value)
}

// Deprecate keys still using an old format in viper configuration by defining
// an alias to the new key, and logging a message describing which keys are
// deprecated as well as the new format to use.
func deprecateFlatKeys(configuration map[string]interface{}, configurationName string) {

	var deprecatedMsg string
	var newValueMsg string
	msgFlatKeyFormat := "\t%q: %T,\n"
	msgNestedKeyFormat := "\t" + msgFlatKeyFormat

	for key, defaultValue := range configuration {
		deprecatedKey := toFlatKey(key)
		if value := viper.Get(deprecatedKey); value != nil {
			subkeys := strings.SplitN(key, ".", 2)
			deprecatedMsg += fmt.Sprintf(msgFlatKeyFormat, deprecatedKey, defaultValue)
			newValueMsg += fmt.Sprintf(msgNestedKeyFormat, subkeys[1], defaultValue)
			// Let viper manage the nested key as the primary key,
			// and the flat key as an alias
			viper.RegisterAlias(deprecatedKey, key)
		}
	}

	if deprecatedMsg != "" {
		log.Printf("Deprecated values are used in configuration file. The following lines:\n%sshould now have this format:\n\t%q:{\n%s\t}",
			deprecatedMsg,
			configurationName,
			newValueMsg)
	}

}

// Returns the flat key corresponding to a nested key.
// For example, for a nested key consul.token, this function will return
// consul_token
func toFlatKey(nestedKey string) string {

	var flatKey string

	// Specific code for keys that don't follow the naming scheme
	if nestedKey == "ansible.operation_remote_base_dir" ||
		nestedKey == "ansible.keep_operation_remote_path" {

		flatKey = strings.Replace(nestedKey, "ansible.", "", 1)
	} else {
		flatKey = strings.Replace(nestedKey, ".", "_", 1)
	}

	return flatKey

}

// Returns the name of the environment variable corresponding to a viper
// nested key. For example, using the prefix yorc,
// nested key consul.token will be associated to YORC_CONSUL_TOKEN environment
// variable
func toEnvVar(key string) string {

	name := environmentVariablePrefix + "_" + toFlatKey(key)
	return strings.ToUpper(name)
}
