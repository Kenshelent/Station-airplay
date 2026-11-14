-- Registration survives for this Lua service's lifetime. No exec/network
-- permission and no runtime credentials are exposed to the HTML page.
register.customweb("index.html")
while true do
  alice.sleep(60000)
end
