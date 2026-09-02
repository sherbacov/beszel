import { Badge } from "@/components/ui/badge"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { InterfaceAddresses, SystemDetailsRecord, SystemRecord } from "@/types"
import { Trans } from "@lingui/react/macro"
import { useMemo } from "react"

/** One row: an interface, with whatever we know about it. */
interface NetworkRow {
	name: string
	/** encapsulation for tunnels, empty for ordinary interfaces */
	kind: string
	/** undefined when the interface is not a tunnel — state is only meaningful there */
	up?: boolean
	addresses: string[]
}

/**
 * Interfaces of one system: tunnels with their encapsulation and state, and the
 * global addresses of every interface.
 *
 * Tunnels are listed even when they carry no address and even when they are
 * down — a tunnel that went down is the thing worth seeing. Ordinary interfaces
 * appear only if they have a global address, otherwise a hypervisor would fill
 * the table with two dozen empty guest taps.
 */
export default function NetworkTable({
	system,
	details,
}: {
	system: SystemRecord
	details?: SystemDetailsRecord
}) {
	const rows = useMemo<NetworkRow[]>(() => {
		const byName = new Map<string, NetworkRow>()

		for (const tunnel of system.info.tun ?? []) {
			byName.set(tunnel.n, { name: tunnel.n, kind: tunnel.k, up: tunnel.u, addresses: [] })
		}
		for (const iface of (details?.addresses ?? []) as InterfaceAddresses[]) {
			const existing = byName.get(iface.n)
			if (existing) {
				existing.addresses = iface.a
			} else {
				byName.set(iface.n, { name: iface.n, kind: "", addresses: iface.a })
			}
		}

		return [...byName.values()].sort((a, b) => {
			// tunnels first — they are the ones with a state worth watching
			const aTunnel = a.kind !== ""
			const bTunnel = b.kind !== ""
			if (aTunnel !== bTunnel) {
				return aTunnel ? -1 : 1
			}
			return a.name.localeCompare(b.name)
		})
	}, [system.info.tun, details?.addresses])

	if (rows.length === 0) {
		return null
	}

	const tunnelCount = rows.filter((row) => row.kind !== "").length
	const downCount = rows.filter((row) => row.up === false).length

	return (
		<Card>
			<CardHeader className="pb-5 px-2 sm:px-6 max-sm:pt-4">
				<CardTitle className="px-2 sm:px-1">
					<Trans>Network Interfaces</Trans>
				</CardTitle>
				<CardDescription className="px-2 sm:px-1">
					{downCount > 0 ? (
						<Trans>
							{tunnelCount} tunnels, {downCount} down
						</Trans>
					) : (
						<Trans>Tunnels and global addresses</Trans>
					)}
				</CardDescription>
			</CardHeader>
			<div className="px-2 sm:px-6 pb-4 overflow-x-auto">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>
								<Trans>Interface</Trans>
							</TableHead>
							<TableHead>
								<Trans>Type</Trans>
							</TableHead>
							<TableHead>
								<Trans>State</Trans>
							</TableHead>
							<TableHead>
								<Trans>Addresses</Trans>
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{rows.map((row) => (
							<TableRow key={row.name}>
								<TableCell className="font-medium whitespace-nowrap">{row.name}</TableCell>
								<TableCell className="text-muted-foreground whitespace-nowrap">
									{row.kind || "—"}
								</TableCell>
								<TableCell>
									{row.up === undefined ? (
										<span className="text-muted-foreground">—</span>
									) : (
										<Badge variant={row.up ? "success" : "danger"}>
											{row.up ? <Trans>up</Trans> : <Trans>down</Trans>}
										</Badge>
									)}
								</TableCell>
								<TableCell className="tabular-nums">
									{row.addresses.length === 0 ? (
										<span className="text-muted-foreground">—</span>
									) : (
										<div className="flex flex-col gap-0.5">
											{row.addresses.map((address) => (
												<span key={address}>{address}</span>
											))}
										</div>
									)}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</div>
		</Card>
	)
}
