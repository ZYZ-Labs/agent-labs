import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.Map;

public class Main {

    private static final String SCHEMA_JSON = """
            {
              "type": "object",
              "properties": {
                "service_name": {"type": "string"},
                "port": {"type": "integer", "minimum": 1024, "maximum": 65535},
                "replicas": {"type": "integer", "minimum": 1, "maximum": 10},
                "env": {"type": "array", "items": {"type": "string"}}
              },
              "required": ["service_name", "port", "replicas"]
            }
            """;

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        String request = "Create a config for a payment-api service on port 8080 with 3 replicas and env vars LOG_LEVEL=info,DB_URL=postgres.";
        Map<String, Object> config = generateWithRecovery(client, mapper, request);
        System.out.println("\nFinal valid config:");
        System.out.println(mapper.writerWithDefaultPrettyPrinter().writeValueAsString(config));
    }

    private static Map<String, Object> generateWithRecovery(OpenAiClient client, ObjectMapper mapper, String request) throws Exception {
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode system = messages.addObject();
        system.put("role", "system");
        system.put("content", "You are a configuration generator. Return only valid JSON matching this schema: "
                + SCHEMA_JSON.replaceAll("\\s+", " ") + ". No markdown, no explanation.");
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", request);

        for (int attempt = 1; attempt <= 3; attempt++) {
            System.out.println("\n--- Attempt " + attempt + " ---");
            ObjectNode payload = mapper.createObjectNode();
            payload.set("messages", messages);
            payload.put("temperature", 0.2);
            payload.put("max_tokens", 300);

            JsonNode response = client.chatCompletion(payload);
            String raw = client.extractMessage(response).get("content").asText();
            System.out.println("Raw output: " + raw);

            Map<String, Object> parsed;
            try {
                parsed = mapper.readValue(raw, Map.class);
            } catch (Exception exc) {
                String error = "Invalid JSON: " + exc.getMessage();
                System.out.println(error);
                messages.add(makeAssistantMessage(mapper, raw));
                messages.add(makeUserMessage(mapper, "That was not valid JSON. " + error + " Please retry."));
                continue;
            }

            String error = validateConfig(parsed);
            if (error == null) {
                return parsed;
            }

            System.out.println("Validation error: " + error);
            messages.add(makeAssistantMessage(mapper, raw));
            messages.add(makeUserMessage(mapper, "Validation failed: " + error + ". Fix the JSON and retry."));
        }

        throw new RuntimeException("Failed to generate valid config after max retries.");
    }

    private static String validateConfig(Map<String, Object> config) {
        if (!(config instanceof Map)) {
            return "Config must be a JSON object.";
        }
        for (String key : new String[]{"service_name", "port", "replicas"}) {
            if (!config.containsKey(key)) {
                return "Missing required field: " + key;
            }
        }
        Object portObj = config.get("port");
        int port = portObj instanceof Number n ? n.intValue() : Integer.parseInt(portObj.toString());
        if (!(1024 <= port && port <= 65535)) {
            return "port must be between 1024 and 65535.";
        }
        Object replicasObj = config.get("replicas");
        int replicas = replicasObj instanceof Number n ? n.intValue() : Integer.parseInt(replicasObj.toString());
        if (!(1 <= replicas && replicas <= 10)) {
            return "replicas must be between 1 and 10.";
        }
        return null;
    }

    private static ObjectNode makeAssistantMessage(ObjectMapper mapper, String content) {
        ObjectNode m = mapper.createObjectNode();
        m.put("role", "assistant");
        m.put("content", content);
        return m;
    }

    private static ObjectNode makeUserMessage(ObjectMapper mapper, String content) {
        ObjectNode m = mapper.createObjectNode();
        m.put("role", "user");
        m.put("content", content);
        return m;
    }
}
