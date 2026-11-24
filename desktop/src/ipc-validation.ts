import Ajv2020 from 'ajv/dist/2020';
import contract from '../../contracts/desktop/ipc.json';

const validators = new Map<string, ReturnType<Ajv2020['compile']>>();
function localSchema(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(localSchema);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value).filter(([key]) => key !== '$id').map(([key, child]) => [key, localSchema(child)]));
}

// Only installed fixed methods call this; the catalog never dispatches a URL.
export function validIPCRequest(method: string, request: unknown): boolean {
  const schemas = contract['x-ipc-requests'] as Record<string, object>;
  if (!Object.hasOwn(schemas, method)) return false;
  let validate = validators.get(method);
  if (!validate) {
    const ajv = new Ajv2020({ strict: false, validateFormats: false });
    validate = ajv.compile(localSchema({ ...schemas[method], components: contract.components }) as object);
    validators.set(method, validate);
  }
  return validate(request) as boolean;
}
