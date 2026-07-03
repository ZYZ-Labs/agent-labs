import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.HashMap;
import java.util.Map;

public class Main {

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        String question = "How does LangGraph help build reliable agents?";
        System.out.println("Question: " + question);
        Map<String, String> result = runGraph(client, question);
        System.out.println("\nAction chosen: " + result.get("action"));
        System.out.println("Context:\n" + result.get("context"));
        System.out.println("\nAnswer: " + result.get("answer"));
    }

    private static Map<String, String> runGraph(OpenAiClient client, String question) throws Exception {
        Map<String, String> state = new HashMap<>();
        state.put("question", question);

        state = merge(state, retrieveContext(state));
        state = merge(state, decideAction(client, state));

        if ("search_more".equals(state.get("action"))) {
            state = merge(state, webSearch(state));
        }

        state = merge(state, generateAnswer(client, state));
        return state;
    }

    private static Map<String, String> merge(Map<String, String> state, Map<String, String> update) {
        Map<String, String> merged = new HashMap<>(state);
        merged.putAll(update);
        return merged;
    }

    private static Map<String, String> retrieveContext(Map<String, String> state) {
        String context = "Documents related to: " + state.get("question") + "\n"
                + "- Agent engineering relies on composable patterns.\n"
                + "- LangGraph adds structure to agent loops with states and edges.";
        System.out.println("Retrieved context for: " + state.get("question"));
        return Map.of("context", context);
    }

    private static Map<String, String> decideAction(OpenAiClient client, Map<String, String> state) throws Exception {
        if (client == null) {
            System.out.println("No LLM; using deterministic action fallback");
            return Map.of("action", "answer_directly");
        }
        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", "You are a routing agent. Given the user question and retrieved context, choose the next action: 'answer_directly' or 'search_more'. Reply with exactly one of those two strings, nothing else.\n\nQuestion: "
                + state.get("question") + "\nContext: " + state.get("context"));
        payload.set("messages", messages);
        payload.put("temperature", 0.0);
        payload.put("max_tokens", 10);

        JsonNode response = client.chatCompletion(payload);
        String action = client.extractMessage(response).get("content").asText().strip().toLowerCase();
        if (!action.equals("answer_directly") && !action.equals("search_more")) {
            action = "answer_directly";
        }
        System.out.println("Decided action: " + action);
        return Map.of("action", action);
    }

    private static Map<String, String> webSearch(Map<String, String> state) {
        System.out.println("Performing mock web search");
        String extra = "\n- Recent web result confirms LangGraph 0.2 adds checkpointing.";
        return Map.of("context", state.get("context") + extra);
    }

    private static Map<String, String> generateAnswer(OpenAiClient client, Map<String, String> state) throws Exception {
        if (client == null) {
            System.out.println("No LLM; using deterministic answer fallback");
            return Map.of("answer", "Answer for '" + state.get("question") + "': based on the retrieved context, LangGraph helps structure agent workflows with states and edges.");
        }
        String prompt = "Use the context below to answer the question concisely.\n\nQuestion: " + state.get("question")
                + "\n\nContext:\n" + state.get("context") + "\n\nAnswer:";
        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.3);
        payload.put("max_tokens", 200);

        JsonNode response = client.chatCompletion(payload);
        return Map.of("answer", client.extractMessage(response).get("content").asText().strip());
    }
}
