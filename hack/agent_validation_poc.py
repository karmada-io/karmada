# Copyright 2026 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import sys
from langchain_ollama import OllamaLLM
from langchain_core.prompts import ChatPromptTemplate

MODEL_NAME = "llama3.2"

def run_validation_loop():
    print("🚀 Starting Karmada AI Agent Validation Guardrail PoC...\n")

    # init model
    try:
        llm = OllamaLLM(model=MODEL_NAME, temperature=0)
    except Exception as e:
        print(f"❌ Error initializing Ollama: {e}. Ensure Ollama is running.")
        sys.exit(1)

    # ground truth
    mock_karmada_state = {
        "active_clusters": ["member1", "member2", "member3"],
        "valid_propagation_policies": ["nginx-propagation", "core-services-policy"]
    }

    # mock hallucinated output
    simulated_generator_output = """
    ### Root Cause Analysis Report
    - **Status**: Critical Failure on workload deployment.
    - **Identified Cluster**: member-99
    - **Applied Policy**: invalid-policy-name
    - **Recommendation**: Scale replicas to 3 on cluster member-99 using override controls.
    """

    print("--- Step 1: Raw Skill Output Received ---")
    print(simulated_generator_output.strip())
    print("\n-----------------------------------------\n")

    # strict validator prompt
    validator_prompt = ChatPromptTemplate.from_messages([
        ("system", """You are a strict data validation bot.

ALLOWED CLUSTERS: {active_clusters}
ALLOWED POLICIES: {valid_policies}

INSTRUCTIONS:
1. Look at the 'Identified Cluster' and 'Applied Policy' in the report.
2. If the identified cluster is NOT in the ALLOWED CLUSTERS list, reply: FAILED - Invalid Cluster.
3. If the applied policy is NOT in the ALLOWED POLICIES list, reply: FAILED - Invalid Policy.
4. If the cluster AND the policy are both in the allowed lists, you MUST reply with exactly one word: PASSED. Do not add any other text."""),
        ("user", "Report to evaluate:\n\n{report}")
    ])

    validator_chain = validator_prompt | llm

    # pass 1
    print("--- Step 2: Running Validator Guardrail (Iteration 1) ---")
    raw_validation = validator_chain.invoke({
        "active_clusters": ", ".join(mock_karmada_state["active_clusters"]),
        "valid_policies": ", ".join(mock_karmada_state["valid_propagation_policies"]),
        "report": simulated_generator_output
    })
    
    validation_result = str(raw_validation).strip()
    print(f"[Validator Evaluation]:\n{validation_result}")
    print("\n-----------------------------------------\n")

    # self-correction
    if validation_result.upper() != "PASSED":
        print("--- Step 3: Routing Feedback Back to Generator for Self-Correction ---")
        
        generator_correction_prompt = ChatPromptTemplate.from_messages([
            ("system", """You are the Karmada RCA AI Specialist. You made an error in your initial assessment and introduced hallucinations. 
Fix your report using the feedback provided by the Validator. Do NOT add extra conversational text. Output ONLY the corrected markdown report.

VALID METADATA TO USE:
- Active Clusters: {active_clusters}
- Valid Policies: {valid_policies}"""),
            ("user", "Original Report:\n{original_report}\n\nValidator Feedback:\n{feedback}\n\nGenerate the corrected clean markdown report:")
        ])

        correction_chain = generator_correction_prompt | llm
        corrected_output = correction_chain.invoke({
            "active_clusters": ", ".join(mock_karmada_state["active_clusters"]),
            "valid_policies": ", ".join(mock_karmada_state["valid_propagation_policies"]),
            "original_report": simulated_generator_output,
            "feedback": validation_result
        })

        print(f"[Corrected Generator Output]:\n{corrected_output.strip()}")
        print("\n-----------------------------------------\n")

        # pass 2
        print("--- Step 4: Final Guardrail Pass (Iteration 2) ---")
        raw_final = validator_chain.invoke({
            "active_clusters": ", ".join(mock_karmada_state["active_clusters"]),
            "valid_policies": ", ".join(mock_karmada_state["valid_propagation_policies"]),
            "report": corrected_output
        })
        
        final_validation = str(raw_final).strip()
        print(f"[Final Validator Evaluation]:\n{final_validation}")
        
        if final_validation.upper() == "PASSED":
            print("\n✅ Validation Successful! Guardrails caught and corrected the drift.")
        else:
            print("\n❌ Guardrail block maintained.")

if __name__ == "__main__":
    run_validation_loop()