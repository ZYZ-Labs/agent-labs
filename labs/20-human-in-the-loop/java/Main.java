import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.HashMap;
import java.util.Map;
import java.util.Scanner;

public class Main {

    private static final Path STATE_FILE = Paths.get("workflow_state.json");

    public static void main(String[] args) throws Exception {
        boolean reset = args.length > 0 && "--reset".equals(args[0]);
        HumanInTheLoopWorkflow workflow = new HumanInTheLoopWorkflow(STATE_FILE);
        if (reset) {
            workflow.reset();
            System.out.println("Workflow state reset.");
            return;
        }

        DestructiveAction action = new DestructiveAction("user_account_42", "delete");
        workflow.run(action);
    }

    static class DestructiveAction {
        final String target;
        final String action;

        DestructiveAction(String target, String action) {
            this.target = target;
            this.action = action;
        }

        String describe() {
            return action + " " + target;
        }
    }

    static class HumanInTheLoopWorkflow {
        private final Path stateFile;
        private Map<String, Object> state;

        HumanInTheLoopWorkflow(Path stateFile) throws Exception {
            this.stateFile = stateFile;
            this.state = load();
        }

        @SuppressWarnings("unchecked")
        private Map<String, Object> load() throws Exception {
            if (Files.exists(stateFile)) {
                return new ObjectMapper().readValue(stateFile.toFile(), Map.class);
            }
            return new HashMap<>();
        }

        private void save() throws Exception {
            Path tmp = stateFile.resolveSibling(stateFile.getFileName().toString() + ".tmp");
            new ObjectMapper().writerWithDefaultPrettyPrinter().writeValue(tmp.toFile(), state);
            Files.move(tmp, stateFile, java.nio.file.StandardCopyOption.REPLACE_EXISTING);
        }

        void reset() throws Exception {
            Files.deleteIfExists(stateFile);
            state = new HashMap<>();
        }

        void requestApproval(DestructiveAction action) throws Exception {
            String stage = (String) state.get("stage");
            if ("approved".equals(stage) || "executed".equals(stage) || "rejected".equals(stage)) {
                return;
            }
            System.out.println("\nDestructive action requested: " + action.describe());
            System.out.println("This action cannot be undone. Please review carefully.");
            state.clear();
            state.put("stage", "awaiting_approval");
            state.put("target", action.target);
            state.put("action", action.action);
            state.put("approved", null);
            save();
        }

        Boolean collectDecision() throws Exception {
            String stage = (String) state.get("stage");
            if (!"awaiting_approval".equals(stage)) {
                return (Boolean) state.get("approved");
            }
            Scanner scanner = new Scanner(System.in);
            while (true) {
                System.out.print("Approve? [y/n]: ");
                String answer = scanner.nextLine().strip().toLowerCase();
                if (answer.equals("y") || answer.equals("yes")) {
                    state.put("approved", true);
                    state.put("stage", "approved");
                    save();
                    return true;
                }
                if (answer.equals("n") || answer.equals("no")) {
                    state.put("approved", false);
                    state.put("stage", "rejected");
                    save();
                    return false;
                }
                System.out.println("Please answer 'y' or 'n'.");
            }
        }

        void execute(DestructiveAction action) throws Exception {
            if ("executed".equals(state.get("stage"))) {
                System.out.println("Action already executed.");
                return;
            }
            if (!Boolean.TRUE.equals(state.get("approved"))) {
                System.out.println("Action not approved; will not execute.");
                return;
            }
            System.out.println("\nExecuting: " + action.describe());
            System.out.println("  -> " + action.target + " has been " + action.action + "d.");
            state.put("stage", "executed");
            state.put("executed_at", LocalDateTime.now().format(DateTimeFormatter.ISO_LOCAL_DATE_TIME));
            save();
        }

        void run(DestructiveAction action) throws Exception {
            requestApproval(action);
            Boolean approved = collectDecision();
            if (Boolean.TRUE.equals(approved)) {
                execute(action);
            } else {
                System.out.println("\nAction rejected by human operator.");
            }
        }
    }
}
