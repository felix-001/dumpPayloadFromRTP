# dumpPayloadFromRTP

wireshark抓到mpeg ps over rtp的包，分析 -> 追踪流 -> tcp流 -> 原始数据 -> 另存为，可以把tcpd的负载dump出来，都是rtp的包。
本程序用于将rtp包的负载dump出来，即是原始mpeg ps数据
