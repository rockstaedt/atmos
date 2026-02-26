#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <Wire.h>
#include <Adafruit_BME280.h>
#include "time.h"

const char* WIFI_SSID = "x"; // 2.4 GHz SSID
const char* WIFI_PASS = "x";

const char* API_URL = "x";
const char* API_TOKEN = "x";

Adafruit_BME280 bme;

const char* NTP_SERVER = "pool.ntp.org";
const long GMT_OFFSET_SEC = 0;
const int DAYLIGHT_OFFSET_SEC = 0;

const unsigned long SEND_INTERVAL_MS = 60000;
unsigned long lastSendMs = 0;

void setup() {
  Serial.begin(115200);
  delay(200);

  Wire.begin();
  bool ok = bme.begin(0x76);
  if (!ok) ok = bme.begin(0x77);
  if (!ok) Serial.println("BME280 not found");

  WiFi.mode(WIFI_STA);
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

String getIsoTimestampUtc() {
  struct tm timeinfo;
  if (!getLocalTime(&timeinfo, 1000)) return "";
  char buf[25];
  strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &timeinfo);
  return String(buf);
}

void loop() {
  if (millis() - lastSendMs >= SEND_INTERVAL_MS) {
    lastSendMs = millis();
    sendMeasurement();
  }
}

void sendMeasurement() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi not connected");
    return;
  }

  WiFiClient clientss;
  if (!clientss.connect("atmos.rockstaedt.de", 443)) {
    Serial.println("TCP connect failed");
    return;
  }
  Serial.println("TCP connected");
  clientss.stop();

  float temperature = bme.readTemperature();
  float humidity = bme.readHumidity();
  float pressure = bme.readPressure() / 100.0f;
  String timestamp = getIsoTimestampUtc();

  String payload = "{";
  payload += "\"temperature\":" + String(temperature, 2) + ",";
  payload += "\"humidity\":" + String(humidity, 2) + ",";
  payload += "\"pressure\":" + String(pressure, 2) + ",";
  payload += "\"room_id\":\"Schlafzimmer\"";
  if (timestamp.length() > 0) {
    payload += ",\"timestamp\":\"" + timestamp + "\"";
  }
  payload += "}";

  WiFiClientSecure client;
  client.setInsecure();
  client.setTimeout(15000);

  HTTPClient https;
  if (!https.begin(client, API_URL)) {
    Serial.println("HTTPS begin failed");
    return;
  }

  https.addHeader("Content-Type", "application/json");
  https.addHeader("Authorization", String("Bearer ") + API_TOKEN);

  int code = https.POST(payload);
  if (code < 0) {
    Serial.printf("HTTP error: %s\n", https.errorToString(code).c_str());
  } else {
    Serial.print("HTTP ");
    Serial.println(code);
    Serial.println(https.getString());
  }

  https.end();
}
