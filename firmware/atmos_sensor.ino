#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <Wire.h>
#include <Adafruit_BME280.h>
#include "time.h"
#include "esp_task_wdt.h"

const char* WIFI_SSID = "x"; // 2.4 GHz SSID
const char* WIFI_PASS = "x";
const char* API_URL = "x";
const char* API_TOKEN = "x";
const char* ROOM_ID = "Schlafzimmer";

Adafruit_BME280 bme;

const char* NTP_SERVER = "pool.ntp.org";
const long GMT_OFFSET_SEC = 0;
const int DAYLIGHT_OFFSET_SEC = 0;

const unsigned long SEND_INTERVAL_MS = 60000;
unsigned long lastSendMs = 0;

void setup() {
  Serial.begin(115200);
  delay(200);

  // Hardware watchdog: reset if stuck for more than 120 seconds
  esp_task_wdt_config_t wdt_config = {
    .timeout_ms    = 120000,
    .idle_core_mask = 0,
    .trigger_panic  = true
  };
  esp_task_wdt_reconfigure(&wdt_config);
  esp_task_wdt_add(NULL);

  Wire.begin();
  bool ok = bme.begin(0x76);
  if (!ok) ok = bme.begin(0x77);
  if (!ok) Serial.println("BME280 not found");

  WiFi.mode(WIFI_STA);
  WiFi.setAutoReconnect(true);
  WiFi.persistent(false); // don't write credentials to flash on every reconnect
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  Serial.print("WiFi connecting");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println(" connected");

  configTime(GMT_OFFSET_SEC, DAYLIGHT_OFFSET_SEC, NTP_SERVER);
  Serial.print("Syncing time");
  struct tm timeinfo;
  while (!getLocalTime(&timeinfo, 10000)) {
    Serial.print(".");
    delay(500);
  }
  Serial.println(" time ok");
}

bool getIsoTimestampUtc(char* buf, size_t len) {
  struct tm timeinfo;
  if (!getLocalTime(&timeinfo, 1000)) return false;
  strftime(buf, len, "%Y-%m-%dT%H:%M:%SZ", &timeinfo);
  return true;
}

void loop() {
  if (millis() - lastSendMs >= SEND_INTERVAL_MS) {
    lastSendMs = millis();
    sendMeasurement();
  }
  esp_task_wdt_reset();
}

void sendMeasurement() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi lost, reconnecting...");
    WiFi.disconnect();
    WiFi.begin(WIFI_SSID, WIFI_PASS);
    unsigned long start = millis();
    while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
      delay(500);
      esp_task_wdt_reset();
    }
    if (WiFi.status() != WL_CONNECTED) {
      Serial.println("Reconnect failed, restarting...");
      ESP.restart();
    }
  }

  float temperature = bme.readTemperature();
  float humidity = bme.readHumidity();
  float pressure = bme.readPressure() / 100.0f;

  char timestamp[25] = "";
  bool hasTimestamp = getIsoTimestampUtc(timestamp, sizeof(timestamp));

  char payload[256];
  if (hasTimestamp) {
    snprintf(payload, sizeof(payload),
      "{\"temperature\":%.2f,\"humidity\":%.2f,\"pressure\":%.2f,\"room_id\":\"%s\",\"timestamp\":\"%s\"}",
      temperature, humidity, pressure, ROOM_ID, timestamp);
  } else {
    snprintf(payload, sizeof(payload),
      "{\"temperature\":%.2f,\"humidity\":%.2f,\"pressure\":%.2f,\"room_id\":\"%s\"}",
      temperature, humidity, pressure, ROOM_ID);
  }

  WiFiClientSecure client;
  client.setInsecure();
  client.setTimeout(15); // seconds (not milliseconds)

  HTTPClient https;
  https.setTimeout(10000); // 10 s HTTP timeout (ms)
  if (!https.begin(client, API_URL)) {
    Serial.println("HTTPS begin failed");
    return;
  }

  https.addHeader("Content-Type", "application/json");
  https.addHeader("Authorization", String("Bearer ") + API_TOKEN);

  int code = https.POST((uint8_t*)payload, strlen(payload));
  if (code < 0) {
    Serial.printf("HTTP error: %s\n", https.errorToString(code).c_str());
  } else {
    Serial.print("HTTP ");
    Serial.println(code);
    Serial.println(https.getString());
  }

  https.end();
}
