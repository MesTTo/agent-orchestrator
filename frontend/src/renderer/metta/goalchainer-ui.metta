;; Select every projected task in one board state.
(@doc gc-ui-task
  (@desc "Returns grounded GoalChainer task records for one MeTTaScript board state")
  (@params
    (@param "The board state to select"))
  (@return "One grounded GoalChainer task record per matching task"))
(: gc-ui-task (-> Atom Atom))
(= (gc-ui-task $state)
  (match &self
    (gc-task-record $task $state $record)
    $record))

;; Select the grounded TypeScript agent record assigned to a task.
(@doc gc-ui-agent-for-task
  (@desc "Returns the Agent Orchestrator agent record assigned to a GoalChainer task")
  (@params
    (@param "The GoalChainer task identity"))
  (@return "The grounded agent record assigned to the task"))
(: gc-ui-agent-for-task (-> Atom Atom))
(= (gc-ui-agent-for-task $task)
  (match &self
    (,
      (gc-agent-task $agent $task)
      (gc-agent-record $agent $record))
    $record))

;; Select every declared project-agent record.
(@doc gc-ui-agent
  (@desc "Returns grounded project-agent records, including agents without a task or stable UUID")
  (@return "One grounded project-agent record per declaration"))
(: gc-ui-agent (-> Atom))
(= (gc-ui-agent)
  (match &self
    (gc-agent-record $agent $record)
    $record))

;; Open the Agent Orchestrator session linked to a task.
(@doc gc-ui-open-session
  (@desc "Invokes the website session navigator for a linked GoalChainer task")
  (@params
    (@param "The GoalChainer task identity"))
  (@return "True when a linked Agent Orchestrator session was opened"))
(: gc-ui-open-session (-> Atom Bool))
(= (gc-ui-open-session $task)
  (match &self
    (gc-task-session $task $session)
    (ao-open-session $session)))

;; Open one immutable AO session attempt linked to a GoalChainer task.
(@doc gc-ui-open-session-attempt
  (@desc "Invokes the session navigator for one Goal and AO attempt record")
  (@params
    (@param "The GoalChainer task identity")
    (@param "The Goal task attempt number"))
  (@return "True when the requested Agent Orchestrator session was opened"))
(: gc-ui-open-session-attempt (-> Atom Number Bool))
(= (gc-ui-open-session-attempt $task $attempt)
  (match &self
    (gc-task-session-attempt $task $attempt $agent-attempt $session)
    (ao-open-session $session)))
