// Agente SNMPv3 que simula un QNAP con QuTS hero 5.2, con la forma de las tablas
// observada en un NAS real (NAS-MIB y QTS-MIB) y datos inventados. Cifra con AES:
// el OpenSSL 3 de Node no trae DES, que es lo que usa QTS (DES se verifica con el NAS real).
import snmp from "net-snmp";

export const QNAP_USER = "talaia-simulado";
export const QNAP_AUTH = "clave-autenticacion-simulada";
export const QNAP_PRIV = "clave-cifrado-simulada";

const NAS = "1.3.6.1.4.1.24681";
const QTS = "1.3.6.1.4.1.55062";
const COMPONENTS = `${NAS}.1.4.1.1.1.1`;
const SPACE = `${NAS}.1.4.1.1.1.2`;
const GiB = 1024 ** 3;
const TiBinKiB = 1024 ** 3;

const { Integer, OctetString, Counter64 } = snmp.ObjectType;
// net-snmp escribe los Counter64 a partir de un Buffer de 8 bytes.
const c64 = (value) => {
  const buffer = Buffer.alloc(8);
  buffer.writeBigUInt64BE(BigInt(Math.round(value)));
  return buffer;
};
const readOnly = snmp.MaxAccess["read-only"];

const scalar = (name, oid, scalarType) => ({ name, type: snmp.MibProviderType.Scalar, oid, scalarType, maxAccess: readOnly });
const table = (name, oid, types) => ({
  name,
  type: snmp.MibProviderType.Table,
  oid,
  maxAccess: snmp.MaxAccess["not-accessible"],
  tableColumns: types.map((type, i) => ({ number: i + 1, name: `${name}${i + 1}`, type, maxAccess: readOnly })),
  tableIndex: [{ columnName: `${name}1` }],
});

const providers = [
  scalar("cpu", `${NAS}.1.3.1`, Integer),
  scalar("memTotal", `${NAS}.1.3.2`, Counter64),
  scalar("memFree", `${NAS}.1.3.3`, Counter64),
  scalar("cpuTemp", `${NAS}.1.3.5`, Integer),
  scalar("sysTemp", `${NAS}.1.3.6`, Integer),
  scalar("hostname", `${QTS}.1.12.4`, OctetString),
  scalar("serial", `${QTS}.1.12.5`, OctetString),
  scalar("firmware", `${QTS}.1.12.6`, OctetString),
  scalar("memAvailable", `${QTS}.1.12.15`, Counter64),
  // diskTable: índice, bahía, caja, resumen, SMART, °C, reserva, modelo, capacidad.
  table("disk", `${COMPONENTS}.5.2.1`, [Integer, Integer, Integer, OctetString, Integer, Integer, Integer, OctetString, Counter64]),
  // systemFan2Table: índice, id, caja, estado, RPM.
  table("fan", `${COMPONENTS}.2.2.1`, [Integer, Integer, Integer, Integer, Integer]),
  // raidGroupTable: índice, id, capacidad, libre, estado, bitmap, nivel.
  table("raid", `${SPACE}.1.2.1`, [Integer, Integer, Counter64, Counter64, OctetString, Integer, OctetString]),
  // poolTable: índice, id, capacidad, libre, estado.
  table("pool", `${SPACE}.2.2.1`, [Integer, Integer, Counter64, Counter64, Integer]),
  // volumeTable: índice, id, capacidad (KiB), libre (KiB), estado, caché SSD, fino, nombre.
  table("volume", `${SPACE}.3.2.1`, [Integer, Integer, Counter64, Counter64, OctetString, Integer, Integer, OctetString]),
];

export function fakeQNAP(port) {
  const values = { smartWarning: false, full: false, degraded: false };
  let agent;

  const fill = () => {
    if (!agent) return;
    const mib = agent.getMib();
    mib.setScalarValue("cpu", 7);
    mib.setScalarValue("memTotal", c64(16 * GiB));
    mib.setScalarValue("memFree", c64(10 * GiB));
    mib.setScalarValue("memAvailable", c64(9 * GiB));
    mib.setScalarValue("cpuTemp", 52);
    mib.setScalarValue("sysTemp", 33);
    mib.setScalarValue("hostname", "nas-simulado");
    mib.setScalarValue("serial", "Q000SIMULADO");
    mib.setScalarValue("firmware", "5.2.9.9999");
    for (const name of ["disk", "fan", "raid", "pool", "volume"]) {
      let rows = [];
      try {
        rows = mib.getTableCells(name, true);
      } catch {
        // La tabla aún no existe: se crea con la primera fila.
      }
      for (const row of rows) mib.deleteTableRow(name, [row[0]]);
    }
    const disks = [
      [1, 1, 0, 35],
      [2, 3, 0, 31],
      [3, 4, 0, 32],
      [4, 5, 0, 30],
      [5, 6, 0, 33],
      [6, 2, 33, -1],
    ];
    for (const [index, bay, enclosure, temp] of disks) {
      const warn = values.smartWarning && bay === 4;
      // Como en el NAS real, una caja USB sin SMART sale «Abnormal», con -1 y sin temperatura.
      const [summary, smart] = enclosure !== 0 ? ["Abnormal", -1] : warn ? ["Warning", 1] : ["Good", 0];
      mib.addTableRow("disk", [index, bay, enclosure, summary, smart, temp, 0, "MODELO-SIMULADO", c64(4 * 1024 * GiB)]);
    }
    mib.addTableRow("fan", [1, 1, 0, 0, 560]);
    mib.addTableRow("raid", [1, 1, c64(11 * 1024 * GiB), c64(0), values.degraded ? "Degraded" : "Ready", 0, "5"]);
    mib.addTableRow("raid", [2, 2, c64(900 * GiB), c64(0), "Ready", 0, "Single"]);
    mib.addTableRow("pool", [1, 1, c64(11 * 1024 * GiB), c64(10 * GiB), values.degraded ? -1 : 0]);
    const free = values.full ? 0.5 * TiBinKiB : 4 * TiBinKiB;
    mib.addTableRow("volume", [1, 1, c64(10 * TiBinKiB), c64(free), "Ready", 1, 0, "Volumen-privado"]);
    mib.addTableRow("volume", [2, 3, c64(4 * TiBinKiB), c64(3 * TiBinKiB), "Ready", -1, 0, "Copias-privadas"]);
  };

  // Cambiar un estado rehace las tablas del agente.
  const state = new Proxy(values, {
    set(target, key, value) {
      target[key] = value;
      fill();
      return true;
    },
  });

  return {
    state,
    url: `snmp://127.0.0.1:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        agent = snmp.createAgent({ port, address: "127.0.0.1", disableAuthorization: false }, (error) => {
          // Los rechazos de autenticación de las pruebas no son un fallo del simulador.
          if (error && !/auth|user|digest|decrypt/i.test(String(error))) console.error("fake-qnap:", error.message);
        });
        agent.getAuthorizer().addUser({
          name: QNAP_USER,
          level: snmp.SecurityLevel.authPriv,
          authProtocol: snmp.AuthProtocols.sha,
          authKey: QNAP_AUTH,
          privProtocol: snmp.PrivProtocols.aes,
          privKey: QNAP_PRIV,
        });
        const mib = agent.getMib();
        mib.registerProviders(providers);
        fill();
        agent.listener?.sockets ? resolve() : setTimeout(resolve, 100);
        agent.on?.("error", reject);
      }),
    stop: () =>
      new Promise((resolve) => {
        if (!agent) return resolve();
        agent.close(resolve);
        agent = undefined;
      }),
  };
}
