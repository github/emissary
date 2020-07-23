local http = require("socket.http")

local function ingress(txn, addr, port)
	txn:set_var("txn.emissary_ingress_success", false)

	local headers = {}
	incoming_headers = txn.http:req_get_headers()

	-- we only care about the x-emissary-auth header on ingress requests
	for i, v in pairs(incoming_headers["x-emissary-auth"]) do
		if headers["x-emissary-auth"] == nil then
			headers["x-emissary-auth"] = v
		else
			headers["x-emissary-auth"] = headers["x-emissary-auth"] .. ", " .. v
		end
	end

	headers['x-emissary-mode'] = "ingress"

	local b, c, h = http.request {
		url = "http://" .. addr .. ":" .. port .. tostring(txn.f:path()),
		headers = headers,
		create = core.tcp,
		redirect = false,
		method = tostring(txn.f:method()),
	}

	if 200 == c then
		txn:set_var("txn.emissary_ingress_success", true)
	end
end

local function egress(txn, addr, port)
	txn:set_var("txn.emissary_egress_success", false)

	local headers = {}
	incoming_headers = txn.http:req_get_headers()

	-- we only care about the host header on egress requests
	for i, v in pairs(incoming_headers["host"]) do
		if headers["host"] == nil then
			headers["host"] = v
		else
			headers["host"] = headers["host"] .. ", " .. v
		end
	end

	headers['x-emissary-mode'] = "egress"

	local b, c, h = http.request {
		url = "http://" .. addr .. ":" .. port .. tostring(txn.f:path()),
		headers = headers,
		create = core.tcp,
		redirect = false,
		method = tostring(txn.f:method()),
	}

	if 200 == c then
		txn:set_var("txn.emissary_egress_success", true)
		txn:set_var("txn.emissary_egress_token", h['x-emissary-auth'])
	end
end

core.register_action("emissary_ingress", { "http-req" }, ingress, 2)
core.register_action("emissary_egress", { "http-req" }, egress, 2)
