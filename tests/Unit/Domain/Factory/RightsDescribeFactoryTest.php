<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Tests\Unit\Domain\Factory;

use App\Domain\Enums\RoleEnums;
use App\Domain\Factory\RightsDescribeFactory;
use App\Domain\ValueObject\RightsDescribe;
use Symfony\Bundle\FrameworkBundle\Test\KernelTestCase;

class RightsDescribeFactoryTest extends KernelTestCase
{

    public function testByRoles(): void
    {
        $factory = new RightsDescribeFactory();

        /** @var array<positive-int, array{roles: array, right_describe: RightsDescribe}> $asserts */
        $asserts = [
            [
                'roles' => [],
                'right_describe' => new RightsDescribe(false)
            ],
            [
                'roles' => [RoleEnums::ROLE_SUPER_ADMIN->value],
                'right_describe' => new RightsDescribe(true)
            ],
            [
                'roles' => [RoleEnums::ROLE_USER->value],
                'right_describe' => new RightsDescribe(false)
            ]
        ];

        foreach ($asserts as $assert) {
            $rightDescribe = $factory->byRoles($assert['roles']);
            $this->assertEquals($assert['right_describe']->isSuperAdmin, $rightDescribe->isSuperAdmin);
        }
    }

}
