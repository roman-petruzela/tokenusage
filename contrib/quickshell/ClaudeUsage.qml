import QtQuick
import Quickshell
import Quickshell.Io

// Shows Claude 5h / 7d usage. Needs the tokenusage-refresh.timer (or drop --cached).
Item {
    id: root
    property real fiveHour: -1
    property real sevenDay: -1
    property string status: "unknown"
    readonly property string label: status === "ok" || status === "stale"
        ? Math.round(fiveHour) + "% · " + Math.round(sevenDay) + "%"
        : "⚠ " + status

    implicitWidth: text.implicitWidth
    implicitHeight: text.implicitHeight

    Text {
        id: text
        text: root.label
        color: root.fiveHour >= 90 ? "#f38ba8" : root.fiveHour >= 70 ? "#f9e2af" : "#a6e3a1"
    }

    Process {
        id: proc
        command: ["tokenusage", "json", "--max-age", "60s"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const d = JSON.parse(text);
                    root.status = d.status;
                    root.fiveHour = d.limits.five_hour ? d.limits.five_hour.utilization : -1;
                    root.sevenDay = d.limits.seven_day ? d.limits.seven_day.utilization : -1;
                } catch (e) {
                    root.status = "parse_error";
                }
            }
        }
    }

    Timer {
        interval: 60000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: proc.running = true
    }
}
