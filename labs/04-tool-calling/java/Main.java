import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.List;
import java.util.Map;

public class Main {

    private static final String TOOLS_JSON = """
            [
              {
                "type": "function",
                "function": {
                  "name": "get_weather",
                  "description": "Get current weather for a city.",
                  "parameters": {
                    "type": "object",
                    "properties": {
                      "city": {"type": "string", "description": "City name"}
                    },
                    "required": ["city"]
                  }
                }
              },
              {
                "type": "function",
                "function": {
                  "name": "search_notes",
                  "description": "Search project notes by keyword.",
                  "parameters": {
                    "type": "object",
                    "properties": {
                      "query": {"type": "string", "description": "Search keyword"}
                    },
                    "required": ["query"]
                  }
                }
              }
            ]
            """;

    private static final List<Map<String, String>> NOTES = List.of(
            Map.of("title", "MCP design", "content", "MCP uses Resources, Tools, Prompts, and Sampling primitives."),
            Map.of("title", "LSP basics", "content", "LSP speaks JSON-RPC over stdio or sockets."),
            Map.of("title", "Agent memory", "content", "Short-term memory lives in the context window; long-term in vectors.")
    );

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();
        String question = "What's the weather in Shanghai? Also, find me notes about MCP.";
        System.out.println("User: " + question);
        String answer = runToolAgent(client, mapper, question);
        System.out.println("\nAssistant: " + answer);
    }

    private static String runToolAgent(OpenAiClient client, ObjectMapper mapper, String userMessage) throws Exception {
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", userMessage);

        ArrayNode tools = (ArrayNode) mapper.readTree(TOOLS_JSON);

        for (int i = 0; i < 5; i++) {
            ObjectNode payload = mapper.createObjectNode();
            payload.set("messages", messages);
            payload.set("tools", tools);
            payload.put("tool_choice", "auto");
            payload.put("temperature", 0.0);
            payload.put("max_tokens", 300);

            JsonNode response = client.chatCompletion(payload);
            JsonNode message = client.extractMessage(response);
            messages.add(message.deepCopy());

            JsonNode toolCalls = message.get("tool_calls");
            if (toolCalls == null || toolCalls.isEmpty()) {
                return message.get("content").asText();
            }

            for (JsonNode toolCall : toolCalls) {
                String name = toolCall.get("function").get("name").asText();
                JsonNode arguments = mapper.readTree(toolCall.get("function").get("arguments").asText());
                System.out.println("[Tool call] " + name + "(" + arguments + ")");

                String result = executeTool(name, arguments, mapper);
                System.out.println("[Tool result] " + result);

                ObjectNode toolResult = messages.addObject();
                toolResult.put("role", "tool");
                toolResult.put("tool_call_id", toolCall.get("id").asText());
                toolResult.put("name", name);
                toolResult.put("content", result);
            }
        }

        return "Reached max iterations.";
    }

    private static String executeTool(String name, JsonNode arguments, ObjectMapper mapper) throws Exception {
        return switch (name) {
            case "get_weather" -> {
                String city = arguments.get("city").asText();
                yield mapper.writeValueAsString(Map.of("city", city, "temperature_c", 22, "condition", "sunny"));
            }
            case "search_notes" -> {
                String query = arguments.get("query").asText().toLowerCase();
                List<Map<String, String>> results = NOTES.stream()
                        .filter(n -> n.get("title").toLowerCase().contains(query)
                                || n.get("content").toLowerCase().contains(query))
                        .toList();
                yield mapper.writeValueAsString(results);
            }
            default -> mapper.writeValueAsString(Map.of("error", "unknown tool " + name));
        };
    }
}
