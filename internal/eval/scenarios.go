package eval

// Scenarios is the full eval suite. UserID is filled by the runner at startup.
var Scenarios = []EvalCase{
	{
		Name:              "account_summary",
		Input:             "What accounts do I have?",
		MustCallTools:     []string{"get_account_summary"},
		OutputMustContain: []string{"HDFC"},
	},
	{
		Name:              "spending_breakdown",
		Input:             "How much did I spend in April 2026?",
		MustCallTools:     []string{"get_spending_breakdown"},
		MaxLLMRounds:      3,
		OutputMustContain: []string{"₹"},
	},
	{
		Name:                   "investment_direct",
		Input:                  "How much did I invest last month?",
		MustCallTools:          []string{"query_transactions"},
		MaxLLMRounds:           6,
		OutputMustContainOneOf: []string{"5,000", "5000", "₹5"},
	},
	{
		Name:              "transactions_list",
		Input:             "Show me my last 5 transactions",
		MustCallTools:     []string{"query_transactions"},
		OutputMustContain: []string{"Zomato", "NACH"},
	},
	{
		Name:              "recurring_list",
		Input:             "What are my active subscriptions?",
		MustCallTools:     []string{"list_recurring"},
		OutputMustContain: []string{"Netflix"},
	},
	{
		Name:          "remember_fact",
		Input:         "Remember that I pay rent of ₹25,000 every month to my landlord",
		MustCallTools: []string{"remember_fact"},
	},
	{
		Name:              "recall_after_remember",
		PreambleInputs:    []string{"Remember that I pay rent of ₹25,000 every month to my landlord"},
		Input:             "What do you know about my rent from your memory?",
		MustCallTools:     []string{"recall_facts"},
		OutputMustContain: []string{"25,000", "rent"},
	},
	{
		Name:                   "label_transaction",
		Input:                  "Show me my last 5 transactions and label the Zomato one as food-delivery",
		MustCallTools:          []string{"query_transactions", "manage_labels"},
		MaxLLMRounds:           5,
		OutputMustContain:      []string{"food-delivery"},
		OutputMustContainOneOf: []string{"added", "labeled", "tagged", "applied"},
	},
	{
		Name:         "max_rounds_respected",
		Input:        "Analyse everything about my finances",
		MaxLLMRounds: 8,
		// No tool or output assertions — just verify agent returns without panic.
	},
	{
		Name:                 "no_hallucinated_accounts",
		Input:                "Do I have a Zerodha account?",
		MaxLLMRounds:         4,
		OutputMustNotContain: []string{"yes, you have a zerodha", "yes you have a zerodha"},
	},
}
