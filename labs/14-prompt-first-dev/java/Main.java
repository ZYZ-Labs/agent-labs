import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final String REQUIREMENT = """
            Build an API for a task manager. Users can create a task with a title and \
            optional description, list all tasks, and mark a task as complete. \
            Store tasks in memory only.
            """;

    private static final String PROMPT_VERSION = System.getenv().getOrDefault("PROMPT_VERSION", "v2");

    private static final Pattern MARKDOWN_BLOCK = Pattern.compile(
            "```markdown\\s*\\n(.*?)\\n```", Pattern.DOTALL);
    private static final Pattern PYTHON_BLOCK = Pattern.compile(
            "```python\\s*\\n(.*?)\\n```", Pattern.DOTALL);

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        String template = loadPromptTemplate(PROMPT_VERSION);
        String prompt = template.replace("{{ requirement }}", REQUIREMENT);

        System.out.println("Using prompt version: " + PROMPT_VERSION);
        System.out.println("Sending " + prompt.length() + " prompt characters to model.");

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.2);
        payload.put("max_tokens", 1500);

        JsonNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).get("content").asText();

        Map.Entry<String, String> artifacts = extractCodeBlocks(content);
        String spec = artifacts.getKey();
        String scaffold = artifacts.getValue();

        Path outputDir = Paths.get("..", "generated");
        writeArtifacts(spec, scaffold, outputDir);

        System.out.println("\n=== Generated API Spec Preview ===");
        System.out.println(preview(spec));
        System.out.println("\n=== Generated Scaffold Preview ===");
        System.out.println(preview(scaffold));
        System.out.println("\nArtifacts written to: " + outputDir.toAbsolutePath());
    }

    private static String loadPromptTemplate(String version) throws Exception {
        Path promptPath = Paths.get("..", "python", "prompts", version, "spec_gen.txt").toAbsolutePath().normalize();
        if (!Files.exists(promptPath)) {
            throw new IllegalArgumentException("Prompt template not found for version '" + version + "' at " + promptPath);
        }
        return Files.readString(promptPath);
    }

    private static Map.Entry<String, String> extractCodeBlocks(String content) {
        Matcher mdMatch = MARKDOWN_BLOCK.matcher(content);
        Matcher pyMatch = PYTHON_BLOCK.matcher(content);
        if (!mdMatch.find() || !pyMatch.find()) {
            throw new IllegalArgumentException("Model response did not contain both required markdown and python code blocks.");
        }
        return Map.entry(mdMatch.group(1).trim(), pyMatch.group(1).trim());
    }

    private static void writeArtifacts(String spec, String scaffold, Path outputDir) throws Exception {
        Files.createDirectories(outputDir);
        Path specPath = outputDir.resolve("api_spec.md");
        Path scaffoldPath = outputDir.resolve("scaffold.py");
        Files.writeString(specPath, spec + "\n");
        Files.writeString(scaffoldPath, scaffold + "\n");
        System.out.println("Wrote artifacts to " + outputDir);
        System.out.println("  - " + specPath.getFileName());
        System.out.println("  - " + scaffoldPath.getFileName());
    }

    private static String preview(String text) {
        int maxChars = 400;
        if (text.length() <= maxChars) {
            return text;
        }
        return text.substring(0, maxChars).stripTrailing() + "\n...";
    }
}
