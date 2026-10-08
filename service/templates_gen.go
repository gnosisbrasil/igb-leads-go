package service

// DefaultTemplate is a seeded message template.
// Content is byte-identical to the Node DEFAULT_TEMPLATES
// (extracted programmatically from MessageTemplateController.js).
type DefaultTemplate struct {
	Key       string
	Label     string
	Content   string
	SortOrder int
	Phase     string
}

// DefaultTemplates by campaign objectives.
var DefaultTemplates = map[string][]DefaultTemplate{
	"camara_publica": {
		{Key: "pedido_confirmacao", Label: "Pedido de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Recebemos sua inscrição para {{tipo_evento}} "{{titulo_campanha}}".

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Por favor, confirme sua presença respondendo esta mensagem.

Até lá!
Equipe Gnosis Brasil`, SortOrder: 1, Phase: "new"},
		{Key: "confirmacao_presenca", Label: "Confirma\u00e7\u00e3o de Presen\u00e7a", Content: `{{saudacao}} {{inscrito_nome}}! ✅

Sua presença em {{tipo_evento}} "{{titulo_campanha}}" está confirmada!

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Estamos ansiosos para vê-lo(a)!

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "lembrete_confirmacao", Label: "Lembrete de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Ainda não recebemos sua confirmação para {{tipo_evento}} "{{titulo_campanha}}".

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Confirme sua presença respondendo esta mensagem.

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "envio_voucher", Label: "Envio de Voucher", Content: `{{saudacao}} {{inscrito_nome}}! 🎫

Seu voucher para {{tipo_evento}} "{{titulo_campanha}}" está pronto!

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Apresente o QR Code da prévia abaixo no dia do evento para o responsável pela turma.

🔗 Voucher: {{url_voucher}}

Equipe Gnosis Brasil`, SortOrder: 3, Phase: "confirmed"},
		{Key: "lembrete_conferencia", Label: "Lembrete da Confer\u00eancia", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Estamos contando os dias para {{tipo_evento}} "{{titulo_campanha}}"!

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 4, Phase: "confirmed"},
		{Key: "envio_motivacao", Label: "Mensagem de Motiva\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}! ✨

Estamos ansiosos para vê-lo(a) em {{tipo_evento}} "{{titulo_campanha}}"!

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 5, Phase: "confirmed"},
		{Key: "lembrete_evento", Label: "Lembrete do Evento", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Lembrete: {{tipo_evento}} "{{titulo_campanha}}" é amanhã!

📅 Data: {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Não se esqueça! Nos vemos lá.
Equipe Gnosis Brasil`, SortOrder: 6, Phase: "confirmed"},
	},
	"workshop": {
		{Key: "pedido_confirmacao", Label: "Pedido de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Recebemos sua inscrição para {{tipo_evento}} "{{titulo_campanha}}".

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Por favor, confirme sua presença respondendo esta mensagem.

Equipe Gnosis Brasil`, SortOrder: 1, Phase: "new"},
		{Key: "confirmacao_presenca", Label: "Confirma\u00e7\u00e3o de Presen\u00e7a", Content: `{{saudacao}} {{inscrito_nome}}! ✅

Sua presença no {{tipo_evento}} "{{titulo_campanha}}" está confirmada!

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Estamos ansiosos para vê-lo(a)!

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "lembrete_confirmacao", Label: "Lembrete de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Ainda não recebemos sua confirmação para {{tipo_evento}} "{{titulo_campanha}}".

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Confirme sua presença respondendo esta mensagem.

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "envio_voucher", Label: "Envio de Voucher", Content: `{{saudacao}} {{inscrito_nome}}! 🎫

Seu voucher para {{tipo_evento}} "{{titulo_campanha}}" está pronto!

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Apresente o QR Code da prévia abaixo no dia do evento para o responsável pela turma.

🔗 Voucher: {{url_voucher}}

Equipe Gnosis Brasil`, SortOrder: 3, Phase: "confirmed"},
		{Key: "lembrete_conferencia", Label: "Lembrete da Confer\u00eancia", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Estamos contando os dias para {{tipo_evento}} "{{titulo_campanha}}"!

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 4, Phase: "confirmed"},
		{Key: "envio_motivacao", Label: "Mensagem de Motiva\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}! ✨

Estamos ansiosos para vê-lo(a) no {{tipo_evento}} "{{titulo_campanha}}"!

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 5, Phase: "confirmed"},
		{Key: "lembrete_evento", Label: "Lembrete do Evento", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Lembrete: {{tipo_evento}} "{{titulo_campanha}}" é amanhã!

📅 Data(s): {{data_evento}}
🕐 Horário: {{hora_evento}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 6, Phase: "confirmed"},
	},
	"primeira_camara": {
		{Key: "pedido_confirmacao", Label: "Pedido de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Recebemos sua inscrição para {{tipo_evento}} "{{titulo_campanha}}".

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Por favor, confirme sua presença respondendo esta mensagem.

Equipe Gnosis Brasil`, SortOrder: 1, Phase: "new"},
		{Key: "confirmacao_presenca", Label: "Confirma\u00e7\u00e3o de Presen\u00e7a", Content: `{{saudacao}} {{inscrito_nome}}! ✅

Sua presença em {{tipo_evento}} "{{titulo_campanha}}" está confirmada!

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Estamos ansiosos para começar!

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "lembrete_confirmacao", Label: "Lembrete de Confirma\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}!

Ainda não recebemos sua confirmação para {{tipo_evento}} "{{titulo_campanha}}".

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Confirme sua presença respondendo esta mensagem.

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "envio_voucher", Label: "Envio de Voucher", Content: `{{saudacao}} {{inscrito_nome}}! 🎫

Seu voucher para {{tipo_evento}} "{{titulo_campanha}}" está pronto!

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Apresente o QR Code da prévia abaixo no dia do evento para o responsável pela turma.

🔗 Voucher: {{url_voucher}}

Equipe Gnosis Brasil`, SortOrder: 3, Phase: "confirmed"},
		{Key: "lembrete_conferencia", Label: "Lembrete da Confer\u00eancia", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Estamos contando os dias para começar {{tipo_evento}} "{{titulo_campanha}}"!

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 4, Phase: "confirmed"},
		{Key: "envio_motivacao", Label: "Mensagem de Motiva\u00e7\u00e3o", Content: `{{saudacao}} {{inscrito_nome}}! ✨

Estamos ansiosos para começar {{tipo_evento}} "{{titulo_campanha}}"!

📅 Início: {{data_evento}}
🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Nos vemos lá!
Equipe Gnosis Brasil`, SortOrder: 5, Phase: "confirmed"},
		{Key: "lembrete_proxima_aula", Label: "Lembrete Pr\u00f3xima Aula", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Lembrete: a próxima aula de {{tipo_evento}} "{{titulo_campanha}}" será conforme o cronograma.

🕐 Horário: {{hora_evento}}
📆 Dias: {{dias_semana}}
📍 {{endereco_completo}}
🗺️ Como chegar: {{google_maps_link}}

Até lá!
Equipe Gnosis Brasil`, SortOrder: 6, Phase: "confirmed"},
	},
	"leads_whatsapp": {
		{Key: "mensagem_inicial", Label: "Mensagem Inicial", Content: `{{saudacao}} {{inscrito_nome}}!

Obrigado pelo seu interesse em {{titulo_campanha}}.

Como podemos ajudá-lo(a)?

Equipe Gnosis Brasil`, SortOrder: 1, Phase: "new"},
		{Key: "confirmacao_presenca", Label: "Confirma\u00e7\u00e3o de Presen\u00e7a", Content: `{{saudacao}} {{inscrito_nome}}! ✅

Sua presença em "{{titulo_campanha}}" está confirmada!

Estamos ansiosos para vê-lo(a)!

Equipe Gnosis Brasil`, SortOrder: 2, Phase: "contacted"},
		{Key: "lembrete_conferencia", Label: "Lembrete da Confer\u00eancia", Content: `{{saudacao}} {{inscrito_nome}}! 🔔

Lembrete sobre {{titulo_campanha}}.

Ainda tem interesse? Ficaremos felizes em ajudá-lo(a).

Equipe Gnosis Brasil`, SortOrder: 3, Phase: "confirmed"},
		{Key: "seguimento", Label: "Mensagem de Seguimento", Content: `{{saudacao}} {{inscrito_nome}}!

Vimos que ainda não respondeu sobre {{titulo_campanha}}.

Ainda tem interesse? Ficaremos felizes em ajudá-lo(a).

Equipe Gnosis Brasil`, SortOrder: 4, Phase: "new"},
	},
}
