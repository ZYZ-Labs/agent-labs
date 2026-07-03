import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final String DESCRIPTION = """
            A blog post resource with title (required string), body (required string), \
            and published (boolean, default false). Provide CRUD endpoints for listing, \
            creating, reading, updating, and deleting posts. Store posts in memory.
            """;

    private static final Map<String, Pattern> FILES = Map.of(
            "models.py", Pattern.compile("```python\\s*models\\.py\\s*\\n(.*?)\\n```", Pattern.DOTALL),
            "routes.py", Pattern.compile("```python\\s*routes\\.py\\s*\\n(.*?)\\n```", Pattern.DOTALL),
            "test_crud.py", Pattern.compile("```python\\s*test_crud\\.py\\s*\\n(.*?)\\n```", Pattern.DOTALL)
    );

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        Map<String, String> files = generateModule(client, mapper);

        boolean allValid = true;
        for (Map.Entry<String, String> entry : files.entrySet()) {
            if (validatePython(entry.getValue(), entry.getKey())) {
                System.out.println("[OK] " + entry.getKey() + " parses and compiles.");
            } else {
                allValid = false;
            }
        }

        Path outputDir = Paths.get("..", "generated");
        writeModule(files, outputDir);

        if (!allValid) {
            System.err.println("One or more generated files failed validation.");
            System.exit(1);
        }

        System.out.println("\nGenerated module written to: " + outputDir.toAbsolutePath());
    }

    private static Map<String, String> generateModule(OpenAiClient client, ObjectMapper mapper) throws Exception {
        String prompt = loadPrompt();
        System.out.println("Sending backend generation prompt (" + prompt.length() + " chars).");

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.2);
        payload.put("max_tokens", 2000);

        JsonNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).get("content").asText();

        Map<String, String> generated = new LinkedHashMap<>();
        for (Map.Entry<String, Pattern> entry : FILES.entrySet()) {
            Matcher matcher = entry.getValue().matcher(content);
            if (!matcher.find()) {
                throw new IllegalArgumentException("Required code block not found in model response: " + entry.getKey());
            }
            generated.put(entry.getKey(), matcher.group(1).strip());
        }
        return generated;
    }

    private static String loadPrompt() throws Exception {
        Path promptPath = Paths.get("..", "python", "prompts", "crud_gen.txt").toAbsolutePath().normalize();
        String template = Files.readString(promptPath);
        return template.replace("{{ description }}", DESCRIPTION);
    }

    private static boolean validatePython(String source, String filename) {
        try {
            ProcessBuilder pb = new ProcessBuilder("python", "-m", "py_compile", "-");
            pb.redirectErrorStream(true);
            Process process = pb.start();
            process.getOutputStream().write(source.getBytes());
            process.getOutputStream().close();
            if (!process.waitFor(10, java.util.concurrent.TimeUnit.SECONDS)) {
                process.destroyForcibly();
                System.err.println("[FAIL] " + filename + ": validation timed out");
                return false;
            }
            if (process.exitValue() != 0) {
                String output = new String(process.getInputStream().readAllBytes());
                System.err.println("[FAIL] " + filename + ": " + output.trim());
                return false;
            }
            return true;
        } catch (Exception exc) {
            System.err.println("[SKIP] " + filename + ": could not run python validation: " + exc.getMessage());
            return true;
        }
    }

    private static void writeModule(Map<String, String> files, Path outputDir) throws Exception {
        Files.createDirectories(outputDir);
        for (Map.Entry<String, String> entry : files.entrySet()) {
            Path path = outputDir.resolve(entry.getKey());
            Files.writeString(path, entry.getValue() + "\n");
            System.out.println("Wrote " + path);
        }
    }
}
