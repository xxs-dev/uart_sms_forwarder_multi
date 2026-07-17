-- =================================================================================
-- PROJECT: UART SMS Forwarder
-- DEVICE:  Air780EPV
-- VERSION: 1.3.0
-- 协议说明：
--   上行（MCU -> 模块）：CMD_START:{json}:CMD_END
--   下行（模块 -> MCU）：SMS_START:{json}:SMS_END
-- =================================================================================

PROJECT = "uart_sms_forwarder"
VERSION = "1.3.0"

log.info("main", PROJECT, VERSION)

-- 1. 引入必要库
sys = require("sys")
sysplus = require("sysplus")

-- 2. 全局配置与变量
-- [注意] 如果接单片机物理引脚，通常是 uart.UART_1；如果是USB调试，用 uart.VUART_0
local uartid = uart.VUART_0
local max_buffer_size = 50
local msg_buffer = {}
local uart_recv_buffer = ""
local call_ring_count = 0  -- 来电响铃计数
local sms_send_queue = {}
local max_sms_queue_size = 20
local legacy_sms_guard_ms = 8000
local traffic_busy = false
local call_forwarding_delays = {[5] = true, [10] = true, [15] = true, [20] = true, [25] = true, [30] = true}

-- 3. 看门狗
if wdt then
    wdt.init(9000)
    sys.timerLoopStart(wdt.feed, 3000)
end

uart.setup(uartid, 115200, 8, 1)
log.info("System", "UART 初始化成功")

-- =================================================================================
-- 工具函数区
-- =================================================================================

local function get_uptime_seconds()
    if mcu and mcu.ticks2 then
        local ok, value = pcall(mcu.ticks2)
        if ok and value then
            return value
        end
    end

    if mcu and mcu.ticks then
        local ok, value = pcall(mcu.ticks)
        if ok and value then
            return math.floor(value / 1000)
        end
    end

    return 0
end

function get_mobile_info()
    local info = {}
    -- 使用 status 判断：0=未注册 1=已注册 2=搜索中 3=拒绝 5=漫游注册
    local net_stat = mobile.status()
    local iccid = mobile.iccid()
    info.sim_ready = (iccid ~= nil and iccid ~= "" and iccid ~= "unknown")
    info.iccid = iccid or "unknown"
    info.imsi = mobile.imsi() or "unknown"
    info.number = mobile.number(0) or ""  -- 获取手机号，可能为空

    -- 获取信号强度指标
    local csq = mobile.csq() or 0 -- 范围 0-31，越大越好
    info.csq = csq
    info.rssi = mobile.rssi() or -113  -- 范围 0到-114，值越大越好
    info.rsrp = mobile.rsrp() or -140  -- 范围 -44到-140，值越大越好 (4G模块)
    info.rsrq = mobile.rsrq() or -20   -- 范围 -3到-19.5，值越大越好 (4G模块)

    -- 根据 CSQ 判断信号等级（仅供参考，4G模块应参考rsrp/rsrq）
    if csq == 0 or csq == 99 then
        info.signal_level = 0
        info.signal_desc = "无信号"
    else
        info.signal_level = csq
        info.signal_desc = csq >= 20 and "强" or (csq >= 10 and "中" or "弱")
    end

    info.is_registered = (net_stat == 1 or net_stat == 5)
    info.is_roaming = net_stat == 5
    info.uptime = get_uptime_seconds() -- 单位为秒

    -- https://docs.openluat.com/osapi/core/mobile/#mobileflymodeindex-enable
    -- 查询飞行模式状态
    -- mobile.flymode() 返回当前飞行模式状态：true 表示飞行模式启用，false 表示飞行模式禁用
    -- 实测永远返回 false？即使飞行模式已启用
    info.flymode = mobile.flymode()

    return info
end

function send_to_uart(data)
    local ok, json_str = pcall(json.encode, data)
    if ok and json_str then
        uart.write(uartid, "SMS_START:" .. json_str .. ":SMS_END\r\n")
        return true
    else
        log.error("UART", "JSON Encode Failed", json_str)
        return false
    end
end

local function send_traffic_result(result)
    result.type = "traffic_result"
    result.timestamp = os.time()
    result.connection_open = false
    send_to_uart(result)
end

local function send_call_forwarding_result(result)
    result.type = "call_forwarding_result"
    result.timestamp = os.time()
    send_to_uart(result)
end

local function normalize_call_forwarding_number(raw_number)
    local number = tostring(raw_number or "")
    local digits = number
    if number:sub(1, 1) == "+" then
        digits = number:sub(2)
    end
    if #digits < 3 or #digits > 20 or not digits:match("^%d+$") then
        return nil, "number must contain an optional + followed by 3 to 20 digits"
    end
    return number, nil
end

local function configure_call_forwarding(request_id, enabled, raw_number, raw_delay)
    local result = {
        request_id = tostring(request_id),
        success = false,
        status = "failed",
        enabled = enabled == true,
        number = tostring(raw_number or ""),
        delay_seconds = tonumber(raw_delay) or 0
    }

    if not cc or type(cc.dial) ~= "function" then
        result.error = "cc.dial API is unavailable in this core"
        send_call_forwarding_result(result)
        return
    end

    local mmi = "##61#"
    if result.enabled then
        local number, number_error = normalize_call_forwarding_number(raw_number)
        if not number then
            result.error = number_error
            send_call_forwarding_result(result)
            return
        end
        local delay = tonumber(raw_delay)
        if not delay or delay ~= math.floor(delay) or not call_forwarding_delays[delay] then
            result.error = "delay must be 5, 10, 15, 20, 25 or 30 seconds"
            send_call_forwarding_result(result)
            return
        end
        result.number = number
        result.delay_seconds = delay
        mmi = "**61*" .. number .. "**" .. tostring(delay) .. "#"
    end

    local call_ok, submitted = pcall(cc.dial, 0, mmi)
    if not call_ok then
        result.error = tostring(submitted)
    elseif submitted ~= true then
        result.error = "modem rejected supplementary service request"
    else
        result.success = true
        result.status = "submitted"
    end
    send_call_forwarding_result(result)
end

local function get_http_client()
    if http and type(http.request) == "function" then
        return http
    end
    return nil
end

local function read_data_traffic()
    if not mobile or type(mobile.dataTraffic) ~= "function" then
        return nil, nil, "mobile.dataTraffic API is unavailable in this core"
    end
    local ok, uplink_gb, uplink_bytes, downlink_gb, downlink_bytes = pcall(mobile.dataTraffic)
    if not ok then
        return nil, nil, "mobile.dataTraffic failed: " .. tostring(uplink_gb)
    end
    if type(uplink_gb) ~= "number" or type(uplink_bytes) ~= "number" or
       type(downlink_gb) ~= "number" or type(downlink_bytes) ~= "number" then
        return nil, nil, "mobile.dataTraffic returned invalid counters"
    end
    local gib = 1024 * 1024 * 1024
    return uplink_gb * gib + uplink_bytes, downlink_gb * gib + downlink_bytes, nil
end

local function perform_traffic_request(request_id, url, target_bytes)
    collectgarbage("collect")
    local result = {
        request_id = request_id,
        success = false,
        http_code = 0,
        uplink_bytes = 0,
        downlink_bytes = 0,
        total_bytes = 0,
        body_bytes = 0,
        target_bytes = target_bytes
    }

    local before_up, before_down, counter_error = read_data_traffic()
    if counter_error then
        result.error = counter_error
        return result
    end

    local http_client = get_http_client()
    if not http_client then
        result.error = "HTTP library is unavailable in this core"
        return result
    end

    local separator = url:find("?", 1, true) and "&" or "?"
    local request_url = url .. separator .. "rid=" .. request_id
    local call_ok, code, _, body = pcall(function()
        return http_client.request(
            "GET",
            request_url,
            {
                ["Connection"] = "close",
                ["Cache-Control"] = "no-cache"
            },
            nil,
            {timeout = 30000}
        ).wait()
    end)

    if call_ok then
        result.http_code = tonumber(code) or 0
        if type(body) == "string" then
            result.body_bytes = #body
        end
    else
        result.error = "HTTP request failed: " .. tostring(code)
    end

    -- Give the modem stack a moment to commit the final TCP ACK/FIN counters.
    body = nil
    sys.wait(1000)
    local after_up, after_down, after_error = read_data_traffic()
    if after_error then
        result.error = after_error
    else
        result.uplink_bytes = math.max(0, after_up - before_up)
        result.downlink_bytes = math.max(0, after_down - before_down)
        result.total_bytes = result.uplink_bytes + result.downlink_bytes
    end

    if call_ok and result.http_code >= 200 and result.http_code < 300 and result.total_bytes > 0 then
        result.success = true
        result.error = nil
    elseif not result.error then
        if result.http_code < 200 or result.http_code >= 300 then
            result.error = "HTTP status " .. tostring(result.http_code)
        else
            result.error = "traffic counters did not increase"
        end
    end

    return result
end

local function run_traffic_request(request_id, url, target_bytes)
    if traffic_busy then
        send_traffic_result({
            request_id = request_id,
            success = false,
            error = "another traffic request is running"
        })
        return
    end
    traffic_busy = true

    local execution_ok, result = pcall(perform_traffic_request, request_id, url, target_bytes)
    traffic_busy = false
    if not execution_ok then
        result = {
            request_id = request_id,
            success = false,
            error = "traffic task crashed: " .. tostring(result)
        }
    end

    collectgarbage("collect")
    send_traffic_result(result)
end

local function sms_error_text(err)
    return tostring(err or "unknown SMS error"):sub(1, 180)
end

local function sms_function(name)
    if sms == nil then
        return nil
    end

    local ok, candidate = pcall(function()
        return sms[name]
    end)
    if ok and type(candidate) == "function" then
        return candidate
    end
    return nil
end

local function has_send_long()
    return sms_function("sendLong") ~= nil
end

local function sms_capabilities()
    local supports_long = has_send_long()
    return {
        send = sms_function("send") ~= nil,
        send_long = supports_long,
        delivery_confirmation = supports_long and "synchronous" or "submission_only",
        legacy_max_content_bytes = supports_long and nil or 140
    }
end

local function normalize_destination(raw_phone)
    local phone = tostring(raw_phone or "")
    phone = phone:gsub("%s+", "")
    phone = phone:gsub("%-", "")
    phone = phone:gsub("%(", "")
    phone = phone:gsub("%)", "")
    phone = phone:gsub("%.", "")

    if phone:sub(1, 2) == "00" then
        phone = "+" .. phone:sub(3)
    end

    local digits = phone
    if phone:sub(1, 1) == "+" then
        digits = phone:sub(2)
    end

    if digits == "" or not digits:match("^%d+$") then
        return nil, nil, "invalid phone number"
    end

    if #phone < 3 or #phone > 20 then
        return nil, nil, "phone number length is invalid"
    end

    return phone, phone:sub(1, 1) == "+", nil
end

local function content_uses_pdu(content)
    for index = 1, #content do
        if content:byte(index) >= 0x80 then
            return true
        end
    end
    return false
end

local function send_sms_result(success, confirmed, delivery_status, request_id, to, err)
    local payload = {
        type = "sms_send_result",
        success = success,
        delivery_confirmed = confirmed,
        delivery_status = delivery_status,
        request_id = tostring(request_id),
        to = to,
        timestamp = os.time()
    }
    if err then
        payload.error = sms_error_text(err)
    end
    send_to_uart(payload)
end

local function send_sms_compat(raw_phone, content)
    if type(content) ~= "string" or content == "" then
        return false, false, "rejected", "SMS content is empty", tostring(raw_phone or "")
    end

    local phone, is_international, phone_error = normalize_destination(raw_phone)
    if not phone then
        return false, false, "rejected", phone_error, tostring(raw_phone or "")
    end

    if has_send_long() then
        local send_long = sms_function("sendLong")
        local call_ok, wait_handle, call_error = pcall(send_long, phone, content, true)
        if not call_ok then
            return false, true, "confirmed", sms_error_text(wait_handle), phone
        end
        if not wait_handle then
            return false, true, "confirmed", sms_error_text(call_error or "sms.sendLong rejected by modem"), phone
        end

        local handle_ok, wait_fn = pcall(function()
            return wait_handle.wait
        end)
        if not handle_ok or type(wait_fn) ~= "function" then
            return false, true, "confirmed", "sms.sendLong returned no wait handle", phone
        end

        local wait_ok, sent = pcall(wait_fn)
        if not wait_ok then
            return false, true, "confirmed", sms_error_text(sent), phone
        end
        if sent == true then
            return true, true, "confirmed", nil, phone
        end
        return false, true, "confirmed", "modem rejected SMS", phone
    end

    -- EC718PV V1002 has only sms.send. It cannot reliably confirm final delivery
    -- and has no long-message API, so keep requests serial and report submission.
    if #content > 140 then
        return false, false, "rejected", "legacy EC718PV firmware supports at most 140 UTF-8 bytes", phone
    end
    local send = sms_function("send")
    if not send then
        return false, false, "rejected", "sms.send API is unavailable", phone
    end

    -- V1002 text mode accepts a full international number when auto-fix is
    -- disabled. Its PDU mode instead requires auto-fix to strip the leading
    -- plus while preserving the country code.
    local auto_phone_fix = true
    if is_international and phone:sub(1, 3) ~= "+86" and not content_uses_pdu(content) then
        auto_phone_fix = false
    end

    local call_ok, submitted = pcall(send, phone, content, auto_phone_fix)
    if not call_ok then
        return false, false, "rejected", sms_error_text(submitted), phone
    end
    if submitted == true then
        return true, false, "submitted", nil, phone
    end
    return false, false, "rejected", "sms.send rejected by modem", phone
end

local function enqueue_sms_request(request_id, to, content)
    if #sms_send_queue >= max_sms_queue_size then
        send_sms_result(false, false, "rejected", request_id, tostring(to or ""), "SMS queue is full")
        return
    end

    table.insert(sms_send_queue, {
        request_id = tostring(request_id),
        to = to,
        content = content
    })
    sys.publish("SMS_SEND_QUEUED")
end

function process_uart_command(cmd_data)
    if not cmd_data.action then
        send_to_uart({type = "error", msg = "missing action"})
        return
    end

    if cmd_data.action == "send_sms" then
        enqueue_sms_request(cmd_data.request_id or os.time(), cmd_data.to, cmd_data.content)

    elseif cmd_data.action == "consume_data" then
        local request_id = tostring(cmd_data.request_id or os.time())
        local request_url = tostring(cmd_data.url or "")
        local target_bytes = tonumber(cmd_data.target_bytes) or 5120
        if request_url == "" then
            send_traffic_result({
                request_id = request_id,
                success = false,
                error = "traffic URL is empty"
            })
        else
            sys.taskInit(run_traffic_request, request_id, request_url, target_bytes)
        end

    elseif cmd_data.action == "configure_call_forwarding" then
        configure_call_forwarding(
            cmd_data.request_id or os.time(),
            cmd_data.enabled == true,
            cmd_data.number,
            cmd_data.delay_seconds
        )

    elseif cmd_data.action == "get_status" then
        send_to_uart({
            type = "status_response",
            timestamp = os.time(),
            mem_kb = math.floor(collectgarbage("count")),
            cellular_enabled = cellular_enabled,
            version = VERSION,
            mobile = get_mobile_info(),
            sms_capabilities = sms_capabilities(),
            traffic_capabilities = {
                http = get_http_client() ~= nil,
                data_traffic = mobile and type(mobile.dataTraffic) == "function",
                sysplus = sysplus ~= nil,
                payload_bytes = 4096
            },
            call_forwarding_capabilities = {
                no_answer = cc ~= nil and type(cc.dial) == "function",
                delays = {5, 10, 15, 20, 25, 30}
            },
            sms_queue_depth = #sms_send_queue
        })

    elseif cmd_data.action == "set_flymode" and cmd_data.enabled ~= nil then
        -- 规范化为布尔值：兼容 true/false、1/0、"true"/"false"
        -- Lua 中 0 也是真值，必须显式转换
        local flymode_enabled = (cmd_data.enabled == true or cmd_data.enabled == 1 or
                                 cmd_data.enabled == "true" or cmd_data.enabled == "1")

        -- 设置飞行模式（0 表示 sim0）
        -- enabled = true 表示启用飞行模式（禁用蜂窝网络）
        -- enabled = false 表示禁用飞行模式（启用蜂窝网络）
        mobile.flymode(0, flymode_enabled)

        send_to_uart({
            type = "cmd_response",
            action = "set_flymode",
            result = "ok"
        })

    elseif cmd_data.action == "reset_stack" then
        log.info("CMD", "重启协议栈")
        mobile.reset()
        send_to_uart({type = "cmd_response", action = "reset_stack", result = "ok"})

    elseif cmd_data.action == "reboot_mcu" then
        log.info("CMD", "重启模块")
        pm.reboot()
        send_to_uart({type = "cmd_response", action = "reboot_mcu", result = "ok"})
    else
        send_to_uart({type = "error", msg = "unknown command"})
    end
end

-- =================================================================================
-- 事件监听区
-- =================================================================================

sys.subscribe("SMS_INC", function(phone, content, metas)
    log.info("Event", "收到短信:", phone)
    local content_hex = ""
    if content and content.toHex then
        content_hex = content:toHex()
    end
    local msg = {
        type = "incoming_sms",
        timestamp = os.time(),
        from = phone,
        content = content,
        content_hex = content_hex,
        metas = metas
    }
    table.insert(msg_buffer, msg)
    if #msg_buffer > max_buffer_size then
        table.remove(msg_buffer, 1) -- 移除旧的
    end
    sys.publish("NEW_MSG_IN_BUFFER")
end)

sys.subscribe("SIM_IND", function(status)
    send_to_uart({type = "sim_event", status = status})
end)

-- 来电事件处理
sys.subscribe("CC_IND", function(state)
    if state == "READY" then
        log.info("Call", "通话准备完成")

    elseif state == "INCOMINGCALL" then
        -- 有电话呼入
        if call_ring_count == 0 then
            log.info("Call", "检测到来电")
            local phone_num = cc.lastNum()
            log.info("Call", "来电号码:", phone_num or "unknown")

            -- 转发来电通知到 UART
            send_to_uart({
                type = "incoming_call",
                timestamp = os.time(),
                from = phone_num or "unknown"
            })
        end

        call_ring_count = call_ring_count + 1

        -- 响4声后自动挂断（可根据需求调整）
--         if call_ring_count > 3 then
--             log.info("Call", "自动挂断来电")
--             cc.hangUp()
--         end

    elseif state == "DISCONNECTED" then
        -- 电话被挂断
        log.info("Call", "通话结束")
        call_ring_count = 0
        send_to_uart({
            type = "call_disconnected",
            timestamp = os.time()
        })
    end
end)

-- =================================================================================
-- 任务循环区
-- =================================================================================

uart.on(uartid, "receive", function(id, len)
    local chunk = uart.read(id, len)
    if not chunk then return end

    uart_recv_buffer = uart_recv_buffer .. chunk

    -- 使用与下行一致的包围标志：CMD_START:{json}:CMD_END
    while true do
        local start_pos = uart_recv_buffer:find("CMD_START:", 1, true)
        if not start_pos then break end

        local end_pos = uart_recv_buffer:find(":CMD_END", start_pos + 10, true)
        if not end_pos then break end  -- 数据未接收完整，等待下次

        -- 提取 JSON 部分
        local json_str = uart_recv_buffer:sub(start_pos + 10, end_pos - 1)
        -- 移除已处理的数据
        uart_recv_buffer = uart_recv_buffer:sub(end_pos + 8)

        if #json_str > 0 then
            local success, cmd = pcall(json.decode, json_str)
            if success and cmd then
                process_uart_command(cmd)
            else
                log.warn("UART", "JSON解析失败:", json_str:sub(1, 50))
                send_to_uart({type="error", msg="Invalid JSON"})
            end
        end
    end

    -- 溢出保护：如果缓冲区过大且找不到有效包，清空
    if #uart_recv_buffer > 4096 then
        log.error("UART", "Buffer Overflow - 清空缓冲区")
        uart_recv_buffer = ""
        send_to_uart({type="error", msg="Buffer overflow, cleared"})
    end
end)

sys.taskInit(function()
    while true do
        if #msg_buffer == 0 then
            sys.waitUntil("NEW_MSG_IN_BUFFER")
        end
        while #msg_buffer > 0 do
            local msg = table.remove(msg_buffer, 1)
            if msg then
                send_to_uart(msg)
                sys.wait(50)
            end
        end
        if collectgarbage("count") > 1024 then
            collectgarbage("collect")
        end
    end
end)

-- One modem can process only one outbound SMS at a time. Serializing requests
-- prevents the legacy EC718PV API from rejecting concurrent sends as busy.
sys.taskInit(function()
    while true do
        if #sms_send_queue == 0 then
            sys.waitUntil("SMS_SEND_QUEUED")
        end

        while #sms_send_queue > 0 do
            local request = table.remove(sms_send_queue, 1)
            local execution_ok, success, confirmed, delivery_status, failure_reason, target = pcall(
                send_sms_compat,
                request.to,
                request.content
            )

            if not execution_ok then
                failure_reason = sms_error_text(success)
                success = false
                confirmed = false
                delivery_status = "error"
                target = tostring(request.to or "")
            end

            log.info("CMD", "SMS result", target, delivery_status)
            send_sms_result(success, confirmed, delivery_status, request.request_id, target, failure_reason)

            if success and delivery_status == "submitted" then
                sys.wait(legacy_sms_guard_ms)
            end
        end
    end
end)

sys.taskInit(function()
    sys.wait(5000)
    send_to_uart({
        type = "system_ready",
        project = PROJECT,
        version = VERSION,
        data_disabled = false,
        background_data_clients = false,
        sms_capabilities = sms_capabilities(),
        call_forwarding_capabilities = {
            no_answer = cc ~= nil and type(cc.dial) == "function",
            delays = {5, 10, 15, 20, 25, 30}
        }
    })
    while true do
        sys.wait(60000)
        local info = get_mobile_info()
        send_to_uart({
            type = "heartbeat",
            rssi = info.rssi,
            signal_level = info.signal_level,
            signal_desc = info.signal_desc,
            net_reg = info.is_registered,
            flymode = info.flymode,
            sim_ready = info.sim_ready,
            mem = math.floor(collectgarbage("count")),
            sms_queue_depth = #sms_send_queue
        })
    end
end)

sys.run()
