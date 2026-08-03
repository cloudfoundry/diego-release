/*
Package gardenhealth contains logic to periodically verify that basic
garden container functionality is working.

gardenhealth.Checker is responsible for executing the healthcheck operation. gardenhealth.Runner
manages the running of the checker and is responsible for communicating health to the executor.

For more details, see the Runner and Checker documentation in this package.
*/
package gardenhealth
