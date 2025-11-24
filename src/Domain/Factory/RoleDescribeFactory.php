<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Factory;

use App\Domain\Enums\RoleEnums;
use App\Domain\ValueObject\RoleDescribe;

class RoleDescribeFactory
{
    /**
     * Проставляет флаги наличия определенных прав
     *
     * @param string[] $roles
     * @return RoleDescribe
     */
    public function byRoles(array $roles): RoleDescribe
    {
        $isSuperAdmin = false;

        if (in_array(RoleEnums::ROLE_SUPER_ADMIN->value, $roles, true)) {
            $isSuperAdmin = true;
        }

        return new RoleDescribe($isSuperAdmin);
    }

}
