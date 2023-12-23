package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
)

var appconf *AppConfig

func initConfig() {
	//Read config:
	configFilename := appname + "_config.json"
	configDir := os.Getenv("CONFDIR")
	if configDir == "" {
		confDirs := []string{
			"/opt/" + appname,
			"/opt/" + appname + "/etc",
			"/usr/lib/" + appname,
			"/usr/lib/" + appname + "/etc",
			"/var/lib/" + appname,
			"/var/lib/" + appname + "/etc",
			"/usr/local/etc",
			"/etc",
			".",
		}
		configDir = "." //the fallback
		for _, cd := range confDirs {
			if _, err := os.Stat(cd + "/" + configFilename); os.IsNotExist(err) { //doesn't exist...
				continue //..so check next one
			}
			configDir = cd
		}
	}
	configFile := configDir + "/" + configFilename
	jsonFile, err := os.Open(configFile)
	if err != nil {
		log.Println("Could not open config file: " + configFile + "\n" + err.Error())
		fmt.Println("Could not open config file: " + configFile + "\n" + err.Error())
	} else {
		defer jsonFile.Close()
		fileBytes, _ := ioutil.ReadAll(jsonFile)

		//strip out // comments from config file:
		re := regexp.MustCompile(`([\s]//.*)|(^//.*)`)
		fileCleanedBytes := re.ReplaceAll(fileBytes, nil)

		err = json.Unmarshal(fileCleanedBytes, &appconf) //populate the config struct with JSON data from the config file
		if err != nil {
			log.Fatal("Could not parse config file: " + configFile + "\n" + err.Error())
		}
	}

	appconf.checkConfig(configFilename)
}

func (f *AppConfig) checkConfig(configFileName string) {
	var invalid bool

	s := reflect.ValueOf(f).Elem()      //the reflected struct
	for i := 0; i < s.NumField(); i++ { //NumField() returns the number of fields in the struct
		fieldValue := s.Field(i) //value of this i'th field
		t := s.Type().Field(i)
		//fmt.Println(t.Name + fmt.Sprintf(" is of kind: %d", t.Type.Kind()))
		if fieldValue.Interface() == "" || fieldValue.Interface() == nil || fieldValue.Interface() == 0 || (t.Type.Kind() != reflect.Bool && fieldValue.IsZero()) { //field is not set already
			//fmt.Println(t.Name + " is empty or zero.")
			if t.Type.Kind() == reflect.String || t.Type.Kind() == reflect.Bool || t.Type.Kind() == reflect.Float64 || t.Type.Kind() == reflect.Int64 || t.Type.Kind() == reflect.Int {
				if !fieldValue.CanSet() {
					log.Printf("Config item '%s' cannot be set!\n", t.Name)
					invalid = true
				} else {
					env, ok := os.LookupEnv(t.Tag.Get("env"))
					if ok && len(env) > 0 {
						//fmt.Println("ENV: " + t.Tag.Get("env") + " is found and is: " + env + " Setting...")
						if err := setField(fieldValue, env); err != nil {
							invalid = true
							log.Println("Error setting '" + t.Name + "' to env '" + env + "'. Error: " + err.Error())
							fmt.Println("Error setting '" + t.Name + "' to env '" + env + "'. Error: " + err.Error())
						} else {
							continue
						}
					} else { //env not found
						//fmt.Println("ENV: '" + t.Tag.Get("env") + "' is NOT FOUND; checking for default value...")
						// Look for user-defined default value
						dflt, ok := t.Tag.Lookup("default")
						//fmt.Println("DEFAULT is: " + dflt)
						if ok {
							if err := setField(fieldValue, dflt); err != nil {
								log.Println("Error setting '" + t.Name + "' to default '" + dflt + "'. Error: " + err.Error())
								fmt.Println("Error setting '" + t.Name + "' to default '" + dflt + "'. Error: " + err.Error())
								invalid = true
							} else {
								continue
							}
						}
					}
				}
			} else {
				log.Printf("Config item '%s' of type %s cannot be set using environment variable %s.", s.Type().Field(i).Name, fieldValue.Type(), s.Type().Field(i).Tag.Get("env"))
				invalid = true
			}

			if !invalid {
				log.Println("===========| ERRORS IN '" + configFileName + "' CONFIG FILE: |===========")
				fmt.Println("--------------------------------------------------------------------------------")
				fmt.Println(" ===========| ERRORS IN '" + configFileName + "' CONFIG FILE: |===========")
				fmt.Println("")
			}
			invalid = true
			log.Printf(" - Required config item '%s' and/or environment variable '%s' is missing or invalid.\n", t.Tag.Get("json"), t.Tag.Get("env"))
			fmt.Printf("      - Required config item '%s' and/or environment variable '%s' is missing or invalid.\n", t.Tag.Get("json"), t.Tag.Get("env"))
		}
	}
	if invalid {
		fmt.Println("--------------------------------------------------------------------------------")
		log.Fatal("Exiting!")
	}
}

func setField(fieldValue reflect.Value, value string) (err error) {
	switch fieldValue.Kind() {
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(value); err != nil {
			return err
		}
		fieldValue.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int64
		if i, err = strconv.ParseInt(value, 0, 64); err != nil {
			return err
		}
		fieldValue.SetInt(int64(i))
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(value, 64); err != nil {
			return err
		}
		fieldValue.SetFloat(f)
	case reflect.String:
		fieldValue.SetString(value)
	default:
		return fmt.Errorf("%s is not a supported config type. Please use bool, float64 float32, int64, int, or string.", fieldValue.Kind())
	}
	return
}
