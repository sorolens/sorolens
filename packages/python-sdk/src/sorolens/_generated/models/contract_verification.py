from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.contract_verification_status import ContractVerificationStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.verification_diagnostic import VerificationDiagnostic
    from ..models.verification_source import VerificationSource
    from ..models.verification_toolchain import VerificationToolchain


T = TypeVar("T", bound="ContractVerification")


@_attrs_define
class ContractVerification:
    """
    Attributes:
        contract_id (str):
        status (ContractVerificationStatus): `verified` when the recompiled hash matches the on-chain hash,
            `failed` otherwise. Diagnostics explain a failure.
        matched (bool):
        source (VerificationSource):
        toolchain (VerificationToolchain):
        diagnostics (list[VerificationDiagnostic]):
        submitted_at (datetime.datetime):
        updated_at (datetime.datetime):
        on_chain_hash (str | Unset): Wasm hash recorded on-chain for the contract (hex).
        compiled_wasm_hash (str | Unset): SHA-256 hash produced by the rebuild (hex).
        build_log (str | Unset): Captured build output, truncated when very large.
        verified_at (datetime.datetime | Unset):
    """

    contract_id: str
    status: ContractVerificationStatus
    matched: bool
    source: VerificationSource
    toolchain: VerificationToolchain
    diagnostics: list[VerificationDiagnostic]
    submitted_at: datetime.datetime
    updated_at: datetime.datetime
    on_chain_hash: str | Unset = UNSET
    compiled_wasm_hash: str | Unset = UNSET
    build_log: str | Unset = UNSET
    verified_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        status = self.status.value

        matched = self.matched

        source = self.source.to_dict()

        toolchain = self.toolchain.to_dict()

        diagnostics = []
        for diagnostics_item_data in self.diagnostics:
            diagnostics_item = diagnostics_item_data.to_dict()
            diagnostics.append(diagnostics_item)

        submitted_at = self.submitted_at.isoformat()

        updated_at = self.updated_at.isoformat()

        on_chain_hash = self.on_chain_hash

        compiled_wasm_hash = self.compiled_wasm_hash

        build_log = self.build_log

        verified_at: str | Unset = UNSET
        if not isinstance(self.verified_at, Unset):
            verified_at = self.verified_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "status": status,
                "matched": matched,
                "source": source,
                "toolchain": toolchain,
                "diagnostics": diagnostics,
                "submitted_at": submitted_at,
                "updated_at": updated_at,
            }
        )
        if on_chain_hash is not UNSET:
            field_dict["on_chain_hash"] = on_chain_hash
        if compiled_wasm_hash is not UNSET:
            field_dict["compiled_wasm_hash"] = compiled_wasm_hash
        if build_log is not UNSET:
            field_dict["build_log"] = build_log
        if verified_at is not UNSET:
            field_dict["verified_at"] = verified_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.verification_diagnostic import (
            VerificationDiagnostic,
        )
        from ..models.verification_source import VerificationSource
        from ..models.verification_toolchain import (
            VerificationToolchain,
        )

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        status = ContractVerificationStatus(d.pop("status"))

        matched = d.pop("matched")

        source = VerificationSource.from_dict(d.pop("source"))

        toolchain = VerificationToolchain.from_dict(d.pop("toolchain"))

        diagnostics = []
        _diagnostics = d.pop("diagnostics")
        for diagnostics_item_data in _diagnostics:
            diagnostics_item = VerificationDiagnostic.from_dict(diagnostics_item_data)

            diagnostics.append(diagnostics_item)

        submitted_at = datetime.datetime.fromisoformat(d.pop("submitted_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        on_chain_hash = d.pop("on_chain_hash", UNSET)

        compiled_wasm_hash = d.pop("compiled_wasm_hash", UNSET)

        build_log = d.pop("build_log", UNSET)

        _verified_at = d.pop("verified_at", UNSET)
        verified_at: datetime.datetime | Unset
        if isinstance(_verified_at, Unset):
            verified_at = UNSET
        else:
            verified_at = datetime.datetime.fromisoformat(_verified_at)

        contract_verification = cls(
            contract_id=contract_id,
            status=status,
            matched=matched,
            source=source,
            toolchain=toolchain,
            diagnostics=diagnostics,
            submitted_at=submitted_at,
            updated_at=updated_at,
            on_chain_hash=on_chain_hash,
            compiled_wasm_hash=compiled_wasm_hash,
            build_log=build_log,
            verified_at=verified_at,
        )

        contract_verification.additional_properties = d
        return contract_verification

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
