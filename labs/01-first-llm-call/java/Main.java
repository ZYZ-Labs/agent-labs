import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

public class Main {
    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        ObjectNode payload = client.buildMessages("Explain what an AI agent is in one sentence.");
        payload.put("max_tokens", 80);

        JsonNode response = client.chatCompletion(payload);
        JsonNode message = client.extractMessage(response);

        System.out.println("Assistant: " + message.get("content").asText());
        System.out.println("Finish reason: " + response.get("choices").get(0).get("finish_reason").asText());
        System.out.println("Usage: " + mapper.writeValueAsString(response.get("usage")));
    }
}
